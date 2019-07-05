/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubeconfig_test

import (
	"encoding/base64"
	"testing"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeconfig"
)

func TestNew(t *testing.T) {
	testcases := []struct {
		name                 string
		controlPlaneEndpoint string
	}{
		{
			name:                 "host:port",
			controlPlaneEndpoint: "192.168.0.2:6443",
		},
		{
			name:                 "https://host:port",
			controlPlaneEndpoint: "https://192.168.0.2:6443",
		},
		{
			name:                 "https://host",
			controlPlaneEndpoint: "https://192.168.0.2",
		},
		{
			name:                 "https://host:port/path",
			controlPlaneEndpoint: "https://192.168.0.2:6443/proxy",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			yaml, err := kubeconfig.New("kubernetes", tc.controlPlaneEndpoint, caKeyPair)
			if err != nil {
				t.Fatal(err)
				return
			}
			t.Log(yaml)
		})
	}
}

var caKeyPair v1alpha1.KeyPair

func init() {
	crt, _ := base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN5ekNDQWJPZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRFNU1EY3dNakUyTlRVMU9Gb1hEVEk1TURZeU9URTJOVFUxT0Zvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTFpnCjVyUXNmSHpEMzVaN0FpcHRKWFFKSUtHWWNQVzhLVi9yQmd2SmZwVlJPeEFDQmtBWVdPZU1KVjdhcFU1UVZ2QlMKY1JDV2c0UVFUWjNSYmRNRnI0K0hmRXhLMFJhczQ2QUQyTzlzdTZjYndzTFBZa09VSnRzaEpQVU5LZzVka21zcAplUGs2T2FUZEszS0krWGNxQ2Y3MzE5aVJZZUhqYXJXNW01U2NFWk41dUJHOTVGNG5XM3l5cjlTaE5Ud1Z4UlRSCkxCd1BTZXB1aTVOR29wYUwrb3pkdERIYmV0UTdTc0U1b29jOHBHOVBBVWlUZWN4TWdjcHEwQU5wN1c5TzhQT00KNm4zZVZJS2NQQUR2dXl0a0JpRTQyZ1F3ZytCZTIrdVMzOWtwY0tyYitZaTFMK3lYKzdOcG0waGFTZXFNVHhTdQpJcHZ0TmREa0VKanQ4SVYvakZNQ0F3RUFBYU1tTUNRd0RnWURWUjBQQVFIL0JBUURBZ0trTUJJR0ExVWRFd0VCCi93UUlNQVlCQWY4Q0FRQXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBRmVRYlF6RUZiSElmQ3NFQURhZHFXdTMKZHBYTTh4M2dNTmxiK3ViNE93UEpkM3hzWDBnait3Vk9HT0dkWEVETTBGeW5NckttNC9IQnNRVlNtOFJxQUxtcwpWYmo1RElxVUQxZFdWa3p4L2UxTkdGNVd0RDR0aDJEaU5VVktndU1PZXFZaFZ2algreWhQeUs0bDVIMk1pZUdDCnQrZklnQ3FsV0MwbGFjRkxMSkd0Ymk2bGpzbVhNREQ5eG9kVkU0STZyMXk3UjNJcnA1MmxVZ3dqMWtlRWZvRU0KU2duVmxXZ2V0MWhFaDVWeitpdTAzb0FlWlFWYW9IK1BGNDhLbmcxM3V3eTFwdE1nSWpmQ1RhUGNodzdMTEtncwpIL1NIdGNXeFdvYk00V1FuUnNJaEpnaDdDU29yN1p6WXJUeVZmbVNsV3g4bCt2aWFqZFFEWG9CVEhUMk9hWEE9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K")
	key, _ := base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBdG1EbXRDeDhmTVBmbG5zQ0ttMGxkQWtnb1podzlid3BYK3NHQzhsK2xWRTdFQUlHClFCaFk1NHdsWHRxbFRsQlc4Rkp4RUphRGhCQk5uZEZ0MHdXdmo0ZDhURXJSRnF6am9BUFk3Mnk3cHh2Q3dzOWkKUTVRbTJ5RWs5UTBxRGwyU2F5bDQrVG81cE4wcmNvajVkeW9KL3ZmWDJKRmg0ZU5xdGJtYmxKd1JrM200RWIzawpYaWRiZkxLdjFLRTFQQlhGRk5Fc0hBOUo2bTZMazBhaWxvdjZqTjIwTWR0NjFEdEt3VG1paHp5a2IwOEJTSk41CnpFeUJ5bXJRQTJudGIwN3c4NHpxZmQ1VWdwdzhBTys3SzJRR0lUamFCRENENEY3YjY1TGYyU2x3cXR2NWlMVXYKN0pmN3MybWJTRnBKNm94UEZLNGltKzAxME9RUW1PM3doWCtNVXdJREFRQUJBb0lCQUgyWS9DaEduckNaZ0FnMAp6RWdGaEk2Rm5xaEt3RGdyRGQ5VmpvTmRqMFFyZmQ1SFZxQ2JZOWFLS0U1TGl4NEVxK3F6QzlSZG8xSzJtNDA4CjVrSTJIekpjSHRpM2lZanhKWTFVT1BpaHpCV0lRN2MvZEsrUGZyYlgrNGliT1VSTllvRlVQVkI5cmFjQS9XUFMKT3VLNjI4bHdTNENFbG9FbXlaMCtwb3RRYzdZNEtNRHRYdzhQaDNVdGI5NGx2aUhnWjVEUFJZMWNZSWY0aTdMVQpQREJ4UE81ekZnMFZGcGRIVlUwQzlJTElmQy9HYWt5ejRWSktCVFpXQmdOc2JocVhEdDhzNnZBNjczanNKN25uCkw2WEthM3lURk9HUVJHdzdHcnJHbHRVcVNiSEhQSWdwaHFpcmtSNjNmeGpMeDBFZm03VzhYY08zcnUydmJ1bzUKdjZySGlQRUNnWUVBd3ZrZHdGTHI4V1MydDFiaWlPMWowZm1hVGVwaXpJTlQ4T3B0RUFQM2J1SEJ5aldRVDZtaApXQURnbDluSGtzWXJrSjZxckJ1dElyZXpKazhibGk0Z1ZMblZ3WCtVeEM5NlRiWndlc3pwbm8xcTYyelFiOG9VCjR3dWZRL2dzeUR6ckIvWWNTTlJLZTQxZERPdFhESHdUeEJBY2U4bUQwWE01VUZuYzFXbUJXUGtDZ1lFQTczYVoKd1FwSUVPVkEzVEdRa1lhYUtmWFFNZHdtWDRJNHoyRTlYTFRNT2dsMWhQYktkdHZjZ2VFNlNOUVovSFpsMnNWLwpZWmhsSERKZDFsQWpzMU1OMUJQWmZMZjgzZkxWVksyZU1nZXh1cHNmdUxEdGFTN2hXeEROeGJxcnlvN3A1RFQwCnNocldQczlRSnVFaVdIb04yMlBRbVJtL1dYVFFHVGN2T2JsM2pxc0NnWUJsOGRHZmpQdjNSTnpRc2lwU3hDVXMKVmlGYldoRjhzN0pCUnFIdC9OVDBjakJjcFhNbVpDQ0xuakhRMURzb3dGdHBDNzFicmtEeDVURlQ5NHNLRkdZdApSdG5BaWcva0lKc0haVHdjeVdYaDFEbXlqVHZUSjh1U0I3S24zR3kxNmp4TjlsNUZxbEtqbFgrdzBLQzhVMmdXClhRSTNxMTgwTmRZaThFbXFnTGIwS1FLQmdRRFBVQng0MVkvaW9MZHhTRkhpeTJkNFlFbm9nTEh3Q2V0cERzUnoKR0V1ZkMwVms0Y3dTN3ZHT3VCRWZzQkQzVXdHSFQyaWljNjlGcEEwOVY1QXcxZnlvMks1M25Vb2NWUG1BSC9kUApWUDMza2dqNmVxSnZaNWpPb0ZPbGxhRFk4clZuVHJseDRHNFBYcWdEb1BGOUs5NEhTL2p5TXlwSUppdHJTUzFuCmlqd0psUUtCZ0VaTWEvMENDNTRyUTFiRUV4VWZrRGoxdDcxSXJCdnRWRFRiTmN1S01TNWl5R3N6WWdlWFk0bXcKVVIxMVZWaWd1OXJTM1Z0amR1T1g1UTNVZzZ0K0Z6WWc0dG9iZ3k2eTVqWG9PMjUxakhpdlpidnpMY0x0ZGVmcApDL2s1VUVSNmlzRU1memZxM0J1ZTZBcnNOQ1J2YnVBWUMrdjNqTE9xN0pTQ29pYmVJUCtzCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==")
	caKeyPair.Cert = crt
	caKeyPair.Key = key
}
