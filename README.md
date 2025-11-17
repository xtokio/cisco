# Cisco

Connect and run Cisco commands on a switch

## Installation

Cisco package
```bash
go get github.com/xtokio/cisco
```

ENV Variables `you will need a Cisco privilege level 15 password`
```bash
export CISCO_USERNAME="your_cisco_username"
export CISCO_PASSWORD="your_cisco_password"
```

Add the dependency to your `main.go` file:

  ```go
 import (
    "fmt"
    "github.com/xtokio/cisco"
  )
  ```

## Usage

```go
package main

import "github.com/xtokio/cisco"

func main() {
  show_version_data, error := cisco.Show_version("my_switch_full_fqdn")
	if error != nil {
		panic(error)
	}

	println(show_version_data["Hardware"])
	println(show_version_data["Version"])
	println(show_version_data["Release"])
	println(show_version_data["SoftwareImage"])
	println(show_version_data["SerialNumber"])
	println(show_version_data["Uptime"])
	println(show_version_data["Restarted"])
	println(show_version_data["ReloadReason"])
	println(show_version_data["Rommon"])


  show_interfaces_data, error := cisco.Show_interfaces("my_switch_full_fqdn")
	if error != nil {
		panic(error)
	}

	for _, interface_data := range show_interfaces_data {
		println(interface_data.Interface)
		println(interface_data.Description)
		println(interface_data.IPAddress)
		println(interface_data.LinkStatus)
		println(interface_data.ProtocolStatus)
		println(interface_data.Hardware)
		println(interface_data.Reliability)
		println(interface_data.TxLoad)
		println(interface_data.RxLoad)
		println(interface_data.Mtu)
		println(interface_data.Duplex)
		println(interface_data.Speed)
		println(interface_data.MediaType)
		println(interface_data.Bandwidth)
		println(interface_data.Delay)
		println(interface_data.Encapsulation)
		println(interface_data.LastInput)
		println(interface_data.LastOutput)
		println(interface_data.OutputHang)
		println(interface_data.QueueStrategy)
		println(interface_data.InputRateBps)
		println(interface_data.OutputRateBps)
		println(interface_data.PacketsInput)
		println(interface_data.PacketsOutput)
		println(interface_data.Runts)
		println(interface_data.Giants)
		println(interface_data.Throttles)
		println(interface_data.InputErrors)
		println(interface_data.OutputErrors)
		println(interface_data.CrcErrors)
		println(interface_data.Collisions)
		println("==========================================")

	}
}
```

## Contributing

1. Fork it (<https://github.com/xtokio/cisco/fork>)
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request

## Contributors

- [Luis Gomez](https://github.com/xtokio) - creator and maintainer
