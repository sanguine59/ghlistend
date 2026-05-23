package notifier

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	dbusObject    = "/org/freedesktop/Notifications"
	dbusInterface = "org.freedesktop.Notifications"
	appName       = "ghlistend"
)

type Notifier struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

type Notification struct {
	Summary string
	Body    string
	URL     string // currently unused — reserved for default-action wiring
}

func New() (*Notifier, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("dbus session bus: %w", err)
	}
	return &Notifier{
		conn: conn,
		obj:  conn.Object(dbusInterface, dbusObject),
	}, nil
}

func (n *Notifier) Close() error {
	if n.conn != nil {
		return n.conn.Close()
	}
	return nil
}

func (n *Notifier) Notify(msg Notification) error {
	call := n.obj.Call(dbusInterface+".Notify", 0,
		appName,
		uint32(0),
		"applications-internet",
		msg.Summary,
		msg.Body,
		[]string{},
		map[string]dbus.Variant{},
		int32(-1),
	)
	return call.Err
}
