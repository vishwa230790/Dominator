package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func (t *rpcType) DisableUpdates(conn *srpc.Conn,
	request dominator.DisableUpdatesRequest,
	reply *dominator.DisableUpdatesResponse) error {
	if conn.Username() == "" {
		t.logger.Printf("DisableUpdates(%s)\n", request.Reason)
	} else {
		t.logger.Printf("DisableUpdates(%s): by %s\n",
			request.Reason, conn.Username())
	}
	return t.herd.DisableUpdates(conn.Username(), request.Reason)
}
