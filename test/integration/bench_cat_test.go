package integrationtest

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mocknet "github.com/ipfs/go-ipfs/p2p/net/mock"
	"github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/thirdparty/unit"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

func BenchmarkCat1MB(b *testing.B) { benchmarkVarCat(b, unit.MB*1) }
func BenchmarkCat2MB(b *testing.B) { benchmarkVarCat(b, unit.MB*2) }
func BenchmarkCat4MB(b *testing.B) { benchmarkVarCat(b, unit.MB*4) }

func benchmarkVarCat(b *testing.B, size int64) {
	data := RandomBytes(size)
	b.SetBytes(size)
	for n := 0; n < b.N; n++ {
		err := benchCat(b, data, instant)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchCat(b *testing.B, data []byte, conf testutil.LatencyConfig) error {
	b.StopTimer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 2

	// create network
	mn, err := mocknet.FullMeshLinked(ctx, numPeers)
	if err != nil {
		return err
	}
	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency: conf.NetworkLatency,
		// TODO add to conf. This is tricky because we want 0 values to be functional.
		Bandwidth: math.MaxInt32,
	})

	peers := mn.Peers()
	if len(peers) < numPeers {
		return errors.New("test initialization error")
	}

	adder, err := core.NewIPFSNode(ctx, core.ConfigOption(MocknetTestRepo(peers[0], mn.Host(peers[0]), conf, core.DHTOption)))
	if err != nil {
		return err
	}
	defer adder.Close()
	catter, err := core.NewIPFSNode(ctx, core.ConfigOption(MocknetTestRepo(peers[1], mn.Host(peers[1]), conf, core.DHTOption)))
	if err != nil {
		return err
	}
	defer catter.Close()

	bs1 := []peer.PeerInfo{adder.Peerstore.PeerInfo(adder.Identity)}
	bs2 := []peer.PeerInfo{catter.Peerstore.PeerInfo(catter.Identity)}

	if err := catter.Bootstrap(core.BootstrapConfigWithPeers(bs1)); err != nil {
		return err
	}
	if err := adder.Bootstrap(core.BootstrapConfigWithPeers(bs2)); err != nil {
		return err
	}

	added, err := coreunix.Add(adder, bytes.NewReader(data))
	if err != nil {
		return err
	}

	b.StartTimer()
	readerCatted, err := coreunix.Cat(catter, added)
	if err != nil {
		return err
	}

	// verify
	var bufout bytes.Buffer
	io.Copy(&bufout, readerCatted)
	if 0 != bytes.Compare(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	return nil
}
