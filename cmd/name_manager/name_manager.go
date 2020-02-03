// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package main

import (
	"github.com/hchauvin/name_manager/pkg/server"
	"github.com/urfave/cli/v2"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"

	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/olekukonko/tablewriter"

	_ "github.com/hchauvin/name_manager/pkg/local_backend"
	_ "github.com/hchauvin/name_manager/pkg/mongo_backend"
)

var (
	version = "dev"
	commit  = "<none>"
	date    = "<unknown>"
)

func getNameManager(c *cli.Context) (name_manager.NameManager, error) {
	return name_manager.CreateFromURL(c.String("backend"))
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
	app.Name = "name_manager"
	app.Usage = "Manage shared test resources with a global lock"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "backend",
			Value: "local://~/.name_manager",
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "hold",
			Usage: "holds a name for a given family, releasing it on Ctl-C",
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				family := c.Args().Get(0)
				cmd := c.Args().Tail()
				name, errc, release, err := nameManager.Hold(family)
				if err != nil {
					return err
				}
				if len(cmd) == 0 {
					// No command given, release on Ctl-C
					fmt.Println(name)
					sig := make(chan os.Signal)
					signal.Notify(sig, os.Interrupt)
					select {
					case <-sig:
					case err := <-errc:
						return err
					}
				} else {
					c := exec.Command(cmd[0], cmd[1:]...)
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr
					if err := c.Run(); err != nil {
						if exitErr, ok := err.(*exec.ExitError); ok {
							os.Exit(exitErr.ExitCode())
						}
						return err
					}
				}
				if err := release(); err != nil {
					return err
				}
				return nil
			},
		},
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
			Name:  "keep_alive",
			Usage: "keeps alive a name",
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				family := c.Args().Get(0)
				name := c.Args().Get(1)
				if family == "" || name == "" {
					return fmt.Errorf("expected arguments to be <family> <name>")
				}
				return nameManager.KeepAlive(family, name)
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
				if family == "" || name == "" {
					return fmt.Errorf("expected arguments to be <family> <name>")
				}
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
		{
			Name:  "serve",
			Usage: "serves a name manager server",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "address",
					Usage: "address to listen to",
					Value: ":9008",
				},
			},
			Action: func(c *cli.Context) error {
				nameManager, err := getNameManager(c)
				if err != nil {
					return err
				}
				address := c.String("address")
				listener, err := net.Listen("tcp", address)
				if err != nil {
					return err
				}
				fmt.Printf("Listening on %s\n", address)
				return server.Serve(listener, nameManager)
			},
		},
		{
			Name: "install",
			Usage: "ensures a name manager server is installed on a Kubernetes cluster",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
