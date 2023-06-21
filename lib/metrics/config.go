// Copyright 2021 The go-kardia Authors
// This file is part of go-kardia.
//
// The go-kardia library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-kardia library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.

package metrics

// Config contains the configuration for the metric collection.
type Config struct {
	Enabled          bool   `toml:",omitempty"`
	EnabledExpensive bool   `toml:",omitempty"`
	HTTP             string `toml:",omitempty"`
	Port             int    `toml:",omitempty"`
}

// DefaultConfig is the default config for metrics used in go-kardia.
var DefaultConfig = Config{
	Enabled:          false,
	EnabledExpensive: false,
	HTTP:             "127.0.0.1",
	Port:             6000,
}
