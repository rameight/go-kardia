/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"flag"
	"github.com/kardiachain/go-kardia/lib/log"
)

// args
type flagArgs struct {
	path  string
	name  string
}

var args flagArgs

func init() {
	flag.StringVar(&args.path, "path", "./", "path to config file")
	flag.StringVar(&args.name, "name", "config", "config file name")
}

func main() {
	flag.Parse()

	// Setups config.
	config, err := Load(args.path, args.name)
	if err != nil {
		panic(err)
	}

	ethNode, err := NewEth(config)
	if err != nil {
		log.Error("Fail to create Eth sub node", "err", err)
		return
	}
	if err := ethNode.Start(); err != nil {
		log.Error("Fail to start Eth sub node", "err", err)
		return
	}

	waitForever()
}

func waitForever() {
	select {}
}
