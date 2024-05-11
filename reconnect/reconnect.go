package reconnect

import (
	"net"
	"time"
)

type myConn struct {
	conn    net.Conn
	network string
	address string
}

func (c *myConn) tryReconnect() error {
	conn, err := net.Dial(c.network, c.address)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *myConn) Read(b []byte) (int, error) {
	n, err := c.conn.Read(b)
	if err != nil {
		err = c.tryReconnect()
		if err != nil {
			return 0, err
		}
		n, err = c.conn.Read(b)
	}
	return n, err
}

func (c *myConn) Write(b []byte) (int, error) {
	n, err := c.conn.Write(b)
	if err != nil {
		err = c.tryReconnect()
		if err != nil {
			return 0, err
		}
		n, err = c.conn.Write(b)
	}
	return n, err
}

func (c *myConn) Close() error {
	return c.conn.Close()
}

func (c *myConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *myConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *myConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *myConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *myConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func Dial(network, address string) (net.Conn, error) {
	conn := &myConn{
		network: network,
		address: address,
	}
	err := conn.tryReconnect()
	if err != nil {
		return nil, err
	}
	return conn, nil
}
