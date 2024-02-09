package lockwatcher

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
)

var (
	dumpedMutex sync.Mutex
	dumpedStack bool
)

func newLockWatcher(lock sync.Locker, options LockWatcherOptions) *LockWatcher {
	if options.CheckInterval < time.Second {
		options.CheckInterval = 5 * time.Second
	}
	if options.LogTimeout < time.Millisecond {
		options.LogTimeout = time.Second
	}
	if options.MaximumTryInterval > options.LogTimeout>>5 {
		options.MaximumTryInterval = options.LogTimeout >> 5
	}
	if options.MinimumTryInterval > options.LogTimeout>>8 {
		options.MinimumTryInterval = options.LogTimeout >> 8
	}
	rstopChannel := make(chan struct{}, 1)
	stopChannel := make(chan struct{}, 1)
	lockWatcher := &LockWatcher{
		LockWatcherOptions: options,
		lock:               lock,
		rstopChannel:       rstopChannel,
		stopChannel:        stopChannel,
	}
	if _, ok := lock.(RWLock); ok {
		go lockWatcher.loop(lockWatcher.rcheck, rstopChannel)
		go lockWatcher.loop(lockWatcher.wcheck, stopChannel)
	} else {
		go lockWatcher.loop(lockWatcher.check, stopChannel)
	}
	return lockWatcher
}

func (lw *LockWatcher) logTimeout(lockType string) {
	dumpedMutex.Lock()
	defer dumpedMutex.Unlock()
	if dumpedStack {
		lw.Logger.Printf("timed out getting %slock\n", lockType)
		return
	}
	dumpedStack = true
	logLine := fmt.Sprintf(
		"timed out getting %slock, first stack trace follows:\n",
		lockType)
	buffer := make([]byte, 1<<20)
	copy(buffer, logLine)
	nBytes := runtime.Stack(buffer[len(logLine):], true)
	lw.Logger.Print(string(buffer[:len(logLine)+nBytes]))
}

func (lw *LockWatcher) check() {
	lockedChannel := make(chan struct{}, 1)
	timer := time.NewTimer(lw.LogTimeout)
	go func() {
		lw.lock.Lock()
		lockedChannel <- struct{}{}
		if lw.Function != nil {
			lw.Function()
		}
		lw.lock.Unlock()
	}()
	select {
	case <-lockedChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return
	case <-timer.C:
	}
	lw.incrementNumLockTimeouts()
	lw.logTimeout("")
	<-lockedChannel
	lw.clearLockWaiting()
	lw.Logger.Println("eventually got lock")
}

func (lw *LockWatcher) clearLockWaiting() {
	lw.statsMutex.Lock()
	defer lw.statsMutex.Unlock()
	lw.stats.WaitingForLock = false
}

func (lw *LockWatcher) clearRLockWaiting() {
	lw.statsMutex.Lock()
	defer lw.statsMutex.Unlock()
	lw.stats.WaitingForRLock = false
}

func (lw *LockWatcher) clearWLockWaiting() {
	lw.statsMutex.Lock()
	defer lw.statsMutex.Unlock()
	lw.stats.WaitingForWLock = false
}

func (lw *LockWatcher) getStats() LockWatcherStats {
	lw.statsMutex.RLock()
	defer lw.statsMutex.RUnlock()
	return lw.stats
}

func (lw *LockWatcher) incrementNumLockTimeouts() {
	lw.statsMutex.Lock()
	defer lw.statsMutex.Unlock()
	lw.stats.NumLockTimeouts++
	lw.stats.WaitingForLock = true
}

func (lw *LockWatcher) incrementNumRLockTimeouts() {
	lw.statsMutex.Lock()
	defer lw.statsMutex.Unlock()
	lw.stats.NumRLockTimeouts++
	lw.stats.WaitingForRLock = true
}

func (lw *LockWatcher) incrementNumWLockTimeouts() {
	lw.statsMutex.Lock()
	defer lw.statsMutex.Unlock()
	lw.stats.NumWLockTimeouts++
	lw.stats.WaitingForWLock = true
}

func (lw *LockWatcher) loop(check func(), stopChannel <-chan struct{}) {
	for {
		timer := time.NewTimer(lw.CheckInterval)
		select {
		case <-stopChannel:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			check()
		}
	}
}

func (lw *LockWatcher) rcheck() {
	lockedChannel := make(chan struct{}, 1)
	timer := time.NewTimer(lw.LogTimeout)
	rwlock := lw.lock.(RWLock)
	go func() {
		rwlock.RLock()
		lockedChannel <- struct{}{}
		if lw.RFunction != nil {
			lw.RFunction()
		}
		rwlock.RUnlock()
	}()
	select {
	case <-lockedChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return
	case <-timer.C:
	}
	lw.incrementNumRLockTimeouts()
	lw.logTimeout("r")
	<-lockedChannel
	lw.clearRLockWaiting()
	lw.Logger.Println("eventually got rlock")
}

// wcheck uses a TryLock() to grab a write lock, so as to not block future read
// lockers.
func (lw *LockWatcher) wcheck() {
	rwlock := lw.lock.(RWLock)
	sleeper := backoffdelay.NewExponential(lw.MinimumTryInterval,
		lw.MaximumTryInterval,
		1)
	timeoutTime := time.Now().Add(lw.LogTimeout)
	for ; time.Until(timeoutTime) > 0; sleeper.Sleep() {
		if rwlock.TryLock() {
			if lw.Function != nil {
				lw.Function()
			}
			rwlock.Unlock()
			return
		}
	}
	lw.incrementNumWLockTimeouts()
	lw.logTimeout("w")
	sleeper = backoffdelay.NewExponential(lw.MaximumTryInterval, time.Second, 1)
	for ; true; sleeper.Sleep() {
		if rwlock.TryLock() {
			if lw.Function != nil {
				lw.Function()
			}
			rwlock.Unlock()
			break
		}
	}
	lw.clearWLockWaiting()
	lw.Logger.Println("eventually got wlock")
}

func (lw *LockWatcher) stop() {
	select {
	case lw.rstopChannel <- struct{}{}:
	default:
	}
	select {
	case lw.stopChannel <- struct{}{}:
	default:
	}
}

func (lw *LockWatcher) writeHtml(writer io.Writer,
	prefix string) (bool, error) {
	stats := lw.GetStats()
	if stats.NumLockTimeouts > 0 {
		if stats.WaitingForLock {
			fmt.Fprintf(writer, "<font color=\"red\">")
		} else {
			fmt.Fprintf(writer, "<font color=\"salmon\">")
		}
		fmt.Fprintf(writer, "%sLock timeouts: %d",
			prefix, stats.NumLockTimeouts)
		if stats.WaitingForLock {
			fmt.Fprintf(writer, " still waiting for lock")
		}
		_, err := fmt.Fprintln(writer, "</font><br>\n")
		return true, err
	}
	if stats.NumRLockTimeouts < 1 && stats.NumWLockTimeouts < 1 {
		return false, nil
	}
	if stats.WaitingForRLock || stats.WaitingForWLock {
		fmt.Fprintf(writer, "<font color=\"red\">")
	} else {
		fmt.Fprintf(writer, "<font color=\"salmon\">")
	}
	if stats.NumRLockTimeouts > 0 {
		fmt.Fprintf(writer,
			"%sRLock timeouts: %d", prefix, stats.NumRLockTimeouts)
		if stats.WaitingForRLock {
			fmt.Fprintf(writer, ", still waiting for RLock")
		}
		prefix = ", "
	}
	if stats.NumWLockTimeouts > 0 {
		fmt.Fprintf(writer,
			"%sWLock timeouts: %d", prefix, stats.NumWLockTimeouts)
		if stats.WaitingForWLock {
			fmt.Fprintf(writer, ", still waiting for WLock")
		}
	}
	_, err := fmt.Fprintln(writer, "</font><br>")
	return true, err
}
