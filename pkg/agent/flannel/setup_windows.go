//go:build windows
// +build windows

package flannel

const (
	cniConfV1 = `{
  "name":"flannel.4096",
  "cniVersion":"1.0.0",
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
	cniConfV2 = `{
  "name":"flannel.4096",
  "cniVersion":"1.0.0",
  "plugins":[
    {
      "type":"flannel",
      "capabilities": {
        "portMappings": true,
        "dns": true
      },
      "delegate": {
        "type": "win-overlay",
        "apiVersion": 2,
        "Policies": [{
            "Name": "EndpointPolicy",
            "Value": {
                "Type": "OutBoundNAT",
                "Settings": {
                  "Exceptions": [
                    "10.42.0.0/16", "10.43.0.0/16"
                  ]
                }
            }
        }, {
            "Name": "EndpointPolicy",
            "Value": {
                "Type": "SDNRoute",
                "Settings": {
                  "DestinationPrefix": "10.43.0.0/16",
                  "NeedEncap": true
                }
            }
        }, {
            "name": "EndpointPolicy",
            "value": {
                "Type": "ProviderAddress",
                "Settings": {
                    "ProviderAddress": "%IPV4_ADDRESS%"
                }
            }
        }]
      }
    }
  ]
}
`
)
