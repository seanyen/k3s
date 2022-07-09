//go:build windows
// +build windows

package flannel

const (
	cniConf = `{
  "name":"flannel.4096",
  "cniVersion":"0.3.1",
  "plugins":[
    {
      "type":"flannel",
      "capabilities": {
        "dns": true
      },
      "delegate": {
        "type": "win-overlay",
        "Policies": [{
            "Name": "EndpointPolicy",
            "Value": {
                "Type": "OutBoundNAT",
                "ExceptionList": ["10.42.0.0/16", "10.43.0.0/16"]
            }
        }, {
            "Name": "EndpointPolicy",
            "Value": {
                "Type": "ROUTE",
                "DestinationPrefix": "10.43.0.0/16",
                "NeedEncap": true
            }
        }]
      }
    }
  ]
}
`
)
