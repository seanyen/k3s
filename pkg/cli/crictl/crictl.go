package crictl

import (
	"os"

	"github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	os.Args = os.Args[1:]
	crictl.Main()
	return nil
}
