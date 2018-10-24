package main

import (
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"strings"

	"github.com/ebfe/scard"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/smartcard-go/lightwallet/actionsets"
)

type commandFunc func(*actionsets.Installer) error

var (
	logger = log.New("package", "status-go/cmd/hardware-wallet-light")

	commands map[string]commandFunc

	flagCommand   = flag.String("c", "", "command")
	flagCapFile   = flag.String("f", "", "cap file path")
	flagOverwrite = flag.Bool("o", false, "overwrite applet if already installed")
	flagLogLevel  = flag.String("l", "", `Log level, one of: "ERROR", "WARN", "INFO", "DEBUG", and "TRACE"`)
)

func initLogger() {
	if *flagLogLevel == "" {
		*flagLogLevel = "info"
	}

	level, err := log.LvlFromString(strings.ToLower(*flagLogLevel))
	if err != nil {
		stdlog.Fatal(err)
	}

	handler := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	filteredHandler := log.LvlFilterHandler(level, handler)
	log.Root().SetHandler(filteredHandler)
}

func init() {
	flag.Parse()
	initLogger()

	commands = map[string]commandFunc{
		"install": commandInstall,
		"status":  commandStatus,
		"delete":  commandDelete,
		"init":    commandInit,
	}
}

func usage() {
	fmt.Printf("\nUsage: hardware-wallet-light COMMAND [FLAGS]\n\nValid commands:\n\n")
	for name := range commands {
		fmt.Printf("- %s\n", name)
	}
	fmt.Print("\nFlags:\n\n")
	flag.PrintDefaults()
	os.Exit(1)
}

func fail(msg string, ctx ...interface{}) {
	logger.Error(msg, ctx...)
	os.Exit(1)
}

func main() {
	if *flagCommand == "" {
		logger.Error("you must specify a command")
		usage()
	}

	ctx, err := scard.EstablishContext()
	if err != nil {
		fail("error establishing card context", "error", err)
	}
	defer func() {
		if err := ctx.Release(); err != nil {
			logger.Error("error releasing context", "error", err)
		}
	}()

	readers, err := ctx.ListReaders()
	if err != nil {
		fail("error getting readers", "error", err)
	}

	if len(readers) == 0 {
		fail("couldn't find any reader")
	}

	if len(readers) > 1 {
		fail("too many readers found")
	}

	reader := readers[0]
	logger.Debug("using reader", "name", reader)
	logger.Debug("connecting to card", "reader", reader)
	card, err := ctx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		fail("error connecting to card", "error", err)
	}
	defer func() {
		if err := card.Disconnect(scard.ResetCard); err != nil {
			logger.Error("error disconnecting card", "error", err)
		}
	}()

	status, err := card.Status()
	if err != nil {
		fail("error getting card status", "error", err)
	}

	switch status.ActiveProtocol {
	case scard.ProtocolT0:
		logger.Debug("card protocol", "T", "0")
	case scard.ProtocolT1:
		logger.Debug("card protocol", "T", "1")
	default:
		logger.Debug("card protocol", "T", "unknown")
	}

	i := actionsets.NewInstaller(card)
	if f, ok := commands[*flagCommand]; ok {
		err = f(i)
		if err != nil {
			logger.Error("error executing command", "command", *flagCommand, "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fail("unknown command", "command", *flagCommand)
	usage()
}

func commandInstall(i *actionsets.Installer) error {
	if *flagCapFile == "" {
		logger.Error("you must specify a cap file path with the -f flag\n")
		usage()
	}

	f, err := os.Open(*flagCapFile)
	if err != nil {
		fail("error opening cap file", "error", err)
	}
	defer f.Close()

	err = i.Install(f, *flagOverwrite)
	if err != nil {
		fail("installation error", "error", err)
	}

	fmt.Printf("applet installed successfully.\n")
	return nil
}

func commandStatus(i *actionsets.Installer) error {
	installed, err := i.Info()
	if err != nil {
		return err
	}

	if installed {
		fmt.Printf("applet already installed\n")
	} else {
		fmt.Printf("applet not installed\n")
	}

	return nil
}

func commandDelete(i *actionsets.Installer) error {
	err := i.Delete()
	if err != nil {
		return err
	}

	fmt.Printf("applet deleted\n")

	return nil
}

func commandInit(i *actionsets.Installer) error {
	secrets, err := i.Init()
	if err != nil {
		return err
	}

	fmt.Printf("PIN %s\n", secrets.Pin())
	fmt.Printf("PUK %s\n", secrets.Puk())
	fmt.Printf("Pairing password: %s\n", secrets.PairingPass())

	return nil
}