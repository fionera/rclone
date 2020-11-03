package minio

import (
	"flag"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
)

func init() {
	flagSet := Command.Flags()
	vfsflags.AddFlags(flagSet)

	flagSet.String("address", ":"+minio.GlobalMinioDefaultPort, "bind to a specific ADDRESS:PORT, ADDRESS can be an IP or hostname")
	flagSet.Bool("quiet", false, "disable startup information")
	flagSet.Bool("anonymous", false, "hide sensitive information from logging")
	flagSet.Bool("json", false, "output server logs and startup information in json format")
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "minio remote:path",
	Short: `Serve the remote over Minio.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)

		f := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			app := cli.NewApp()

			flagSet := flag.NewFlagSet("", flag.ContinueOnError)
			command.Flags().VisitAll(func(f *pflag.Flag) {
				switch f.Value.Type() {
				case "string":
					s, err := command.Flags().GetString(f.Name)
					if err != nil {
						panic(err)
					}
					flagSet.String(f.Name, s, "")
				case "bool":
					b, err := command.Flags().GetBool(f.Name)
					if err != nil {
						panic(err)
					}
					flagSet.Bool(f.Name, b, "")
				}
			})

			ctx := cli.NewContext(app, flagSet, nil)

			minio.StartGateway(ctx, fsGateway{f})

			return nil
		})
	},
}

type fsGateway struct {
	fs.Fs
}

func (f fsGateway) NewGatewayLayer(creds auth.Credentials) (minio.ObjectLayer, error) {
	return NewFSLayer(creds, vfs.New(f, &vfsflags.Opt))
}

func (f fsGateway) Production() bool {
	return true
}
