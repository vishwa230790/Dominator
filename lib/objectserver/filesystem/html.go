package filesystem

import (
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

func (objSrv *ObjectServer) writeHtml(writer io.Writer) {
	objSrv.lockWatcher.WriteHtml(writer, "ObjectServer: ")
	free, capacity, err := objSrv.getSpaceMetrics()
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	utilisation := float64(capacity-free) * 100 / float64(capacity)
	objSrv.rwLock.RLock()
	duplicatedBytes := objSrv.duplicatedBytes
	numObjects := uint64(len(objSrv.objects))
	numDuplicated := objSrv.numDuplicated
	numReferenced := objSrv.numReferenced
	numUnreferenced := objSrv.numUnreferenced
	referencedBytes := objSrv.referencedBytes
	totalBytes := objSrv.totalBytes
	unreferencedBytes := objSrv.unreferencedBytes
	objSrv.rwLock.RUnlock()
	referencedUtilisation := float64(referencedBytes) * 100 / float64(capacity)
	totalUtilisation := float64(totalBytes) * 100 / float64(capacity)
	unreferencedObjectsPercent := 0.0
	unreferencedUtilisation := float64(unreferencedBytes) * 100 /
		float64(capacity)
	if numObjects > 0 {
		unreferencedObjectsPercent =
			100.0 * float64(numUnreferenced) / float64(numObjects)
	}
	unreferencedBytesPercent := 0.0
	if totalBytes > 0 {
		unreferencedBytesPercent =
			100.0 * float64(unreferencedBytes) / float64(totalBytes)
	}
	fmt.Fprintf(writer,
		"Number of objects: %d, consuming %s (%.1f%% of FS which is %.1f%% full)<br>\n",
		numObjects, format.FormatBytes(totalBytes), totalUtilisation,
		utilisation)
	if numDuplicated > 0 {
		fmt.Fprintf(writer,
			"Number of referenced objects: %d (%d duplicates, %.3g*), consuming %s (%.1f%% of FS, %s dups, %.3g*)<br>\n",
			numReferenced, numDuplicated,
			float64(numDuplicated)/float64(numReferenced),
			format.FormatBytes(referencedBytes),
			referencedUtilisation,
			format.FormatBytes(duplicatedBytes),
			float64(duplicatedBytes)/float64(referencedBytes))
	}
	fmt.Fprintf(writer,
		"Number of unreferenced objects: %d (%.1f%%), consuming %s (%.1f%%, %.1f%% of FS)<br>\n",
		numUnreferenced, unreferencedObjectsPercent,
		format.FormatBytes(unreferencedBytes), unreferencedBytesPercent,
		unreferencedUtilisation)
	if numReferenced+numUnreferenced != numObjects {
		fmt.Fprintf(writer,
			"<font color=\"red\">Object accounting error: ref+unref:%d != total: %d</font><br>\n",
			numReferenced+numUnreferenced, numObjects)
	}
	if referencedBytes+unreferencedBytes != totalBytes {
		fmt.Fprintf(writer,
			"<font color=\"red\">Storage accounting error: ref+unref:%s != total: %s</font><br>\n",
			format.FormatBytes(referencedBytes+unreferencedBytes),
			format.FormatBytes(totalBytes))
	}
}
