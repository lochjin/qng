package crawl

import (
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/cmd/relaynode/config"
	"github.com/Qitmeer/qng/meerevm/eth"
	"github.com/Qitmeer/qng/meerevm/meer"
	"github.com/Qitmeer/qng/services/common"
	ecommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/urfave/cli/v2"
)

func meerCmd() *cli.Command {
	var qd *CrawlService
	return &cli.Command{
		Name:        "meercrawl",
		Aliases:     []string{"mc"},
		Category:    "crawl",
		Usage:       "Updates a nodes.json file with random nodes found in the DHT for Meer",
		Description: "Updates a nodes.json file with random nodes found in the DHT for Meer",
		Flags: []cli.Flag{
			bootnodesFlag,
			nodedbFlag,
			crawlTimeoutFlag,
		},
		Before: func(ctx *cli.Context) error {
			return config.Conf.Load()
		},
		Action: func(ctx *cli.Context) error {
			cfg := config.Conf
			qcfg := common.DefaultConfig(".")
			qcfg.DataDir = cfg.DataDir
			ecfg, err := meer.MakeConfig(qcfg)
			if err != nil {
				return err
			}
			qd = NewCrawlService(cfg, ecfg, ctx)
			return qd.Start()
		},
		After: func(ctx *cli.Context) error {
			if qd != nil {
				return qd.Stop()
			}
			return nil
		},
	}
}

func meerNodesCmd() *cli.Command {
	return &cli.Command{
		Name:        "meernodes",
		Aliases:     []string{"mn"},
		Category:    "crawl",
		Usage:       "Show nodes found in the DHT for Meer from nodes.json file",
		Description: "Show nodes found in the DHT for Meer from nodes.json file",
		Before: func(ctx *cli.Context) error {
			return config.Conf.Load()
		},
		Action: func(ctx *cli.Context) error {
			cfg := config.Conf
			qcfg := common.DefaultConfig(".")
			qcfg.DataDir = cfg.DataDir
			ecfg, err := meer.MakeConfig(qcfg)
			if err != nil {
				return err
			}
			nodesFile := getNodesFilePath(cfg.DataDir, ecfg.Node.Name)
			if !ecommon.FileExist(nodesFile) {
				return fmt.Errorf("Can't find nodes file:%s", nodesFile)
			}
			ns, err := loadNodesJSON(nodesFile)
			if err != nil {
				return err
			}
			for id, n := range ns {
				log.Info("node", "id", id.String(), "ip", n.N.IPAddr().String(), "tcp", n.N.TCP(), "udp", n.N.UDP(), "url", n.N.String())
			}
			log.Info("Finished node", "count", len(ns))
			return nil
		},
		After: func(ctx *cli.Context) error {
			return nil
		},
	}
}

func meerDNSNodesCmd() *cli.Command {
	return &cli.Command{
		Name:        "meerdnsnodes",
		Aliases:     []string{"mdn"},
		Category:    "crawl",
		Usage:       "Show meer DNS nodes",
		Description: "Show meer DNS nodes",
		Before: func(ctx *cli.Context) error {
			return config.Conf.Load()
		},
		Action: func(ctx *cli.Context) error {
			cfg := config.Conf
			qcfg := common.DefaultConfig(".")
			qcfg.DataDir = cfg.DataDir

			ecfg, err := meer.MakeConfig(qcfg)
			if err != nil {
				return err
			}
			eth.SetDNSDiscoveryDefaults(ecfg)
			if len(ecfg.Eth.EthDiscoveryURLs) <= 0 {
				return errors.New("EthDiscoveryURLs is empty")
			}
			var dcfg dnsdisc.Config
			ddClient := dnsdisc.NewClient(dcfg)

			for _, url := range ecfg.Eth.EthDiscoveryURLs {
				domain, _, err := dnsdisc.ParseURL(url)
				if err != nil {
					return err
				}

				t, err := ddClient.SyncTree(url)
				if err != nil {
					return err
				}
				log.Info(domain, "total", len(t.Nodes()))
				for _, n := range t.Nodes() {
					fmt.Println(n.String())
					fmt.Println(n.URLv4())
					fmt.Println()
				}
			}
			return nil
		},
		After: func(ctx *cli.Context) error {
			return nil
		},
	}
}
