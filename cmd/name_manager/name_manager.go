package main

import (
	"log"
	"os"

	"github.com/urfave/cli"

	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/olekukonko/tablewriter"

	_ "github.com/hchauvin/name_manager/pkg/local_backend"
)

var (
	version = "dev"
	commit  = "<none>"
	date    = "<unknown>"
)

func getNameManager(c *cli.Context) (name_manager.NameManager, error) {
	return name_manager.CreateFromURL(c.GlobalString("backend"))
}

func printNames(names []name_manager.Name) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Family", "Created At", "Updated At", "Free"})
	for _, name := range names {
		updatedAtStr := ""
		if !name.UpdatedAt.Equal(name.CreatedAt) {
			updatedAtStr = humanize.Time(name.UpdatedAt)
		}
		freeStr := ""
		if name.Free {
			freeStr = "X"
		}
		table.Append([]string{
			name.Name,
			name.Family,
			humanize.Time(name.CreatedAt),
			updatedAtStr,
			freeStr,
		})
	}
	table.Render()
}

func main() {
	app := cli.NewApp()

	app.Version = fmt.Sprintf("%s (commit: %s; date: %s)", version, commit, date)
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "backend",
			Value: "local://~/.name_manager",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:  "acquire",
			Usage: "acquires a name for a given family",
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				family := c.Args().Get(0)
				name, err := nameManager.Acquire(family)
				if err != nil {
					return err
				}
				fmt.Print(name)
				return nil
			},
		},
		{
			Name:  "release",
			Usage: "releases a name",
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				family := c.Args().Get(0)
				name := c.Args().Get(1)
				return nameManager.Release(family, name)
			},
		},
		{
			Name:  "list",
			Usage: "lists all names",
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				names, err := nameManager.List()
				if err != nil {
					return err
				}
				printNames(names)
				return nil
			},
		},
		{
			Name:  "reset",
			Usage: "resets the backend",
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				return nameManager.Reset()
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
