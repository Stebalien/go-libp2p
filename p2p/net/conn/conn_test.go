package conn

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	travis "github.com/ipfs/go-libp2p/testutil/ci/travis"
	msgio "gx/ipfs/QmRQhVisS8dmPbjBUthVkenn81pBxrx1GxE281csJhm2vL/go-msgio"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func msgioWrap(c Conn) msgio.ReadWriter {
	return msgio.NewReadWriter(c)
}

func testOneSendRecv(t *testing.T, c1, c2 Conn) {
	mc1 := msgioWrap(c1)
	mc2 := msgioWrap(c2)

	log.Debugf("testOneSendRecv from %s to %s", c1.LocalPeer(), c2.LocalPeer())
	m1 := []byte("hello")
	if err := mc1.WriteMsg(m1); err != nil {
		t.Fatal(err)
	}
	m2, err := mc2.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(m1, m2) {
		t.Fatal("failed to send: %s %s", m1, m2)
	}
}

func testNotOneSendRecv(t *testing.T, c1, c2 Conn) {
	mc1 := msgioWrap(c1)
	mc2 := msgioWrap(c2)

	m1 := []byte("hello")
	if err := mc1.WriteMsg(m1); err == nil {
		t.Fatal("write should have failed", err)
	}
	_, err := mc2.ReadMsg()
	if err == nil {
		t.Fatal("read should have failed", err)
	}
}

func TestClose(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c1, c2, _, _ := setupSingleConn(t, ctx)

	testOneSendRecv(t, c1, c2)
	testOneSendRecv(t, c2, c1)

	c1.Close()
	testNotOneSendRecv(t, c1, c2)

	c2.Close()
	testNotOneSendRecv(t, c2, c1)
	testNotOneSendRecv(t, c1, c2)
}

func TestCloseLeak(t *testing.T) {
	// t.Skip("Skipping in favor of another test")
	if testing.Short() {
		t.SkipNow()
	}

	if travis.IsRunning() {
		t.Skip("this doesn't work well on travis")
	}

	var wg sync.WaitGroup

	runPair := func(num int) {
		ctx, cancel := context.WithCancel(context.Background())
		c1, c2, _, _ := setupSingleConn(t, ctx)

		mc1 := msgioWrap(c1)
		mc2 := msgioWrap(c2)

		for i := 0; i < num; i++ {
			b1 := []byte(fmt.Sprintf("beep%d", i))
			mc1.WriteMsg(b1)
			b2, err := mc2.ReadMsg()
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(b1, b2) {
				panic(fmt.Errorf("bytes not equal: %s != %s", b1, b2))
			}

			b2 = []byte(fmt.Sprintf("boop%d", i))
			mc2.WriteMsg(b2)
			b1, err = mc1.ReadMsg()
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(b1, b2) {
				panic(fmt.Errorf("bytes not equal: %s != %s", b1, b2))
			}

			<-time.After(time.Microsecond * 5)
		}

		c1.Close()
		c2.Close()
		cancel() // close the listener
		wg.Done()
	}

	var cons = 5
	var msgs = 50
	log.Debugf("Running %d connections * %d msgs.\n", cons, msgs)
	for i := 0; i < cons; i++ {
		wg.Add(1)
		go runPair(msgs)
	}

	log.Debugf("Waiting...\n")
	wg.Wait()
	// done!

	time.Sleep(time.Millisecond * 150)
	ngr := runtime.NumGoroutine()
	if ngr > 25 {
		// note, this is really innacurate
		//panic("uncomment me to debug")
		t.Fatal("leaking goroutines:", ngr)
	}
}
