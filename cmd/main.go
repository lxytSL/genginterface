package main

import (
	"log"
	code "main/src"
	"os"
	"sort"

	"github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:        "geng",
		Usage:       "gen file",
		Description: "自动生成golang工程依赖文件",
		Commands: []*cli.Command{
			{
				Name:  "geng",
				Usage: "生成文件",
				Subcommands: []*cli.Command{
					{
						Name:    "interface",
						Aliases: []string{"i"},
						Usage:   "生成interface文件",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "d,dir",
								Usage: "指定Go代码路径",
								Value: ".",
							},
						},
						Action: func(c *cli.Context) error {
							code.MakeGenFile(c.String("dir"))
							return nil
						},
					},
				},
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
