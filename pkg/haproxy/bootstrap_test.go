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

package haproxy_test

import (
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/haproxy"
	"sigs.k8s.io/cluster-api/test/framework"
)

func TestBootstrapDataForLoadBalancer(t *testing.T) {
	g := gomega.NewWithT(t)
	bootstrapData, err := haproxy.BootstrapDataForLoadBalancer(
		infrav1.HAProxyLoadBalancer{
			TypeMeta: metav1.TypeMeta{
				Kind:       framework.TypeToKind(&infrav1.HAProxyLoadBalancer{}),
				APIVersion: infrav1.GroupVersion.String(),
			},
			Spec: infrav1.HAProxyLoadBalancerSpec{
				User: &infrav1.SSHUser{
					Name:           "capv",
					AuthorizedKeys: []string{"publicKey"},
				},
			},
		},
		[]byte("client"),
		[]byte("cert"),
		[]byte(testSigningCACertPEMString),
		[]byte(testSigningCAKeyString))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(string(bootstrapData)).To(gomega.Equal(testExpectedBootstrapData))
}

const (
	testSigningCACertPEMString = `-----BEGIN CERTIFICATE-----
MIIDpTCCAo2gAwIBAgIJAMXCj3EmByuLMA0GCSqGSIb3DQEBBQUAMGUxCzAJBgNV
BAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlQYWxvIEFsdG8x
DzANBgNVBAoMBlZNd2FyZTENMAsGA1UECwwEQ0FQVjENMAsGA1UEAwwEY2FwdjAe
Fw0xOTEyMjMyMjA0MDZaFw0yOTEyMjAyMjA0MDZaMGUxCzAJBgNVBAYTAlVTMRMw
EQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlQYWxvIEFsdG8xDzANBgNVBAoM
BlZNd2FyZTENMAsGA1UECwwEQ0FQVjENMAsGA1UEAwwEY2FwdjCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAOIWfe9L7nOMcBNdmWWvtyVB/nKrVujVP9yO
ahs9dG5avWrSKp6TZqQqHyQunz2P1j/3eUNBVsB7q2iac7lvEas8i8gvM6cnZjCy
sPunOpcHTtmZ7cklW2xJk/67g7CderMHUFlEKZ2Gg0dKy1GHT0K60xBpbbcr7ULH
bAsSWfpQqr3IoLJVIz0VIkyZoFDc8Mq8yWFxTFpdM53tIQ5tRJbFpXVq1FKR0QnM
fSgpl3BXHf9hqoiEwtt0NLaTa8oj33gfAd9JdkJyQGMxinRFNGITlS63I3rdAr2E
bTZ6pV1XnGbOwfQjDMPB4eR7LH97IF+nrpPyp+Idz0whGMrFATUCAwEAAaNYMFYw
CQYDVR0TBAIwADALBgNVHQ8EBAMCBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG
AQUFBwMBMB0GA1UdDgQWBBRPgsHYMwkkUtbI3emNlA8M/GJ6VTANBgkqhkiG9w0B
AQUFAAOCAQEAbfyRDthXiLECmCI9cQe6Q9wMSSupqwuRfYZjMPcWfKiqTSlzug2z
K5i0DaksZczkSabZQY4C2Dhc4IY2WvDZE6CErMmMvWgbC68Uy3fJiyyxYZslA79R
7tBqNyjZ/uD/3hlxC+tj6W6K01g8pZnftJLqm1PbobPTOzn4OObPfb8rUrWUvN+N
mICeqNzl9NaOylKo6KtpZrd6w0+AEBhN0O7/3VB2smu/iwCvusSAX0kqiK5r0m6f
M1H3ksI6jzuHbl4DzhiOGyUpPKczHsG9KWid58Z3/JWl86J4jE1yt8zdAP7fk+dO
4OET19pMmMHYg9NKRW1HpQhUbx3oOhEv1g==
-----END CERTIFICATE-----
`

	testSigningCAKeyString = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA4hZ970vuc4xwE12ZZa+3JUH+cqtW6NU/3I5qGz10blq9atIq
npNmpCofJC6fPY/WP/d5Q0FWwHuraJpzuW8RqzyLyC8zpydmMLKw+6c6lwdO2Znt
ySVbbEmT/ruDsJ16swdQWUQpnYaDR0rLUYdPQrrTEGlttyvtQsdsCxJZ+lCqvcig
slUjPRUiTJmgUNzwyrzJYXFMWl0zne0hDm1ElsWldWrUUpHRCcx9KCmXcFcd/2Gq
iITC23Q0tpNryiPfeB8B30l2QnJAYzGKdEU0YhOVLrcjet0CvYRtNnqlXVecZs7B
9CMMw8Hh5Hssf3sgX6euk/Kn4h3PTCEYysUBNQIDAQABAoIBAQCNtbdd5GQjvOUK
3mIl4IuVKNZKHact7WxH3GQZit2NxgZwDCd2mcF+KIC4dxiMx7ltArrZMv0jTODV
geoDUuDqSdr7sMpZfVKKN5bDRcBtpcEAl4D50RaKu1uuEO6sJykfSfhM23KSMBvc
9b6W7Y76rotZABwq8beXYdQQ5IHNaNDTvNvrmtOpVnzHjwcw0ZWMLqwipcmuhLot
vm9RriOcIJqAT13LAkp4+29fpP5RMTmNseTYZryN9UKg1BToeP8jVALxgx48yxLE
tYVHN3TIq5pNk7D1CJ/HAcMhxEL3D5St0RGCv7GzGWC5+Wk486V/tnhpvKIJPI+5
nShBsuxBAoGBAPXDP9/8qDVOFuhwdo9a+T3rOCN2iXNlAndJgCLdv8Kf1u92o8hu
mKLGvnSbnt67zVWIZWSSmstCn8zSYNj96BkCmHiIXrhAeeBQPJsf60kMBnrYgs/Z
gOqzH9JOw1NcnIkyhZVSjInn39rWFiKTWI7CRyeRdC4IMxQU/vQLw+JNAoGBAOuB
b12UcQtF5EzK5nxweA55IwCff5VTiwRoHX2FyTUSahn9K6S8P/FN+VQMu5fsauek
60gHHfLuXAYijQ1Km4bdgZyJhVLN+WzNHmDJGQsuXPRJ+idHTy0R39MCSxcNSonQ
+TisMyg3EEIKVJ24J4OlgPusH7NPv6yhm85tS36JAoGBAOLb6QqJ33vVKbBGoCqU
f554ksmpkhfDFhOm9XE54Nl3UqCZk3ZhIOShMQ3S2UQhd9mMnovICLu4NGqNiHjF
aIotqzEYMNdELTyy1D8dp8M2JoUfdyEGVcpQrv8jVYqN4rGCwWylVrW2JR2MocIo
4YZmL+iGjAgx6XSQLQh6E8fBAoGAbQW/i1/DsUdKt+4aIzNhsLmNZaVwx60kJwcX
19sOWV5L9foIsTtgkpHZQXqfgWY120SykuaQi7yip0hpaeTG+PkkHlZffQTTWfXf
AUk3KcDt0T1J69MMKT4kEqf2IRbLEd/G7+Bv0kcjZJ8pqtXsnPoKKvf0uOrLPdyW
p0pbb5kCgYBTs+/RbqdeTpbkKXBcaYYcIpr9+HOiaGLl1LHhuOPzU8AqV5mOXg4d
jidtLwlOklv6wUvMVJMchUjJJHpY9kn6E++3QvB29BcfK85fP9oKf0WB0Lp0z13G
t+d3Y+TAco6lzgo1lIUn/LxxPodeLiCN04saLy7jGlUmWQF53lSN2Q==
-----END RSA PRIVATE KEY-----
`

	testExpectedBootstrapData = `## template: jinja
#cloud-config

write_files:
- path: /etc/haproxy/haproxy.cfg
  owner: haproxy:haproxy
  permissions: "0640"
  content: |
    global
    log    /dev/log  local0
    log    /dev/log  local1 notice
    chroot /var/lib/haproxy
    stats  socket /run/haproxy.sock mode 660 level admin expose-fd listeners
    stats  timeout 30s
    user   haproxy
    group  haproxy
    stats  socket /run/haproxy.sock user haproxy group haproxy mode 660 level admin
    master-worker

    ca-base /etc/ssl/certs
    crt-base /etc/ssl/private

    ssl-default-bind-ciphers ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS
    ssl-default-bind-options no-sslv3

    defaults
    log     global
    mode    http
    option  httplog
    option  dontlognull
        timeout connect 5000
        timeout client  50000
        timeout server  50000

    userlist controller
    user client insecure-password cert

    program api
    command dataplaneapi --scheme=https --haproxy-bin=/usr/sbin/haproxy --config-file=/etc/haproxy/haproxy.cfg --reload-cmd="/usr/bin/systemctl restart haproxy" --reload-delay=5 --tls-host=0.0.0.0 --tls-port=5556 --tls-ca=/etc/haproxy/ca.crt --tls-certificate=/etc/haproxy/server.crt --tls-key=/etc/haproxy/server.key --userlist=controller
    no option start-on-reload

- path: /etc/haproxy/ca.crt
  owner: haproxy:haproxy
  permissions: "0640"
  content: |
    -----BEGIN CERTIFICATE-----
    MIIDpTCCAo2gAwIBAgIJAMXCj3EmByuLMA0GCSqGSIb3DQEBBQUAMGUxCzAJBgNV
    BAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlQYWxvIEFsdG8x
    DzANBgNVBAoMBlZNd2FyZTENMAsGA1UECwwEQ0FQVjENMAsGA1UEAwwEY2FwdjAe
    Fw0xOTEyMjMyMjA0MDZaFw0yOTEyMjAyMjA0MDZaMGUxCzAJBgNVBAYTAlVTMRMw
    EQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlQYWxvIEFsdG8xDzANBgNVBAoM
    BlZNd2FyZTENMAsGA1UECwwEQ0FQVjENMAsGA1UEAwwEY2FwdjCCASIwDQYJKoZI
    hvcNAQEBBQADggEPADCCAQoCggEBAOIWfe9L7nOMcBNdmWWvtyVB/nKrVujVP9yO
    ahs9dG5avWrSKp6TZqQqHyQunz2P1j/3eUNBVsB7q2iac7lvEas8i8gvM6cnZjCy
    sPunOpcHTtmZ7cklW2xJk/67g7CderMHUFlEKZ2Gg0dKy1GHT0K60xBpbbcr7ULH
    bAsSWfpQqr3IoLJVIz0VIkyZoFDc8Mq8yWFxTFpdM53tIQ5tRJbFpXVq1FKR0QnM
    fSgpl3BXHf9hqoiEwtt0NLaTa8oj33gfAd9JdkJyQGMxinRFNGITlS63I3rdAr2E
    bTZ6pV1XnGbOwfQjDMPB4eR7LH97IF+nrpPyp+Idz0whGMrFATUCAwEAAaNYMFYw
    CQYDVR0TBAIwADALBgNVHQ8EBAMCBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG
    AQUFBwMBMB0GA1UdDgQWBBRPgsHYMwkkUtbI3emNlA8M/GJ6VTANBgkqhkiG9w0B
    AQUFAAOCAQEAbfyRDthXiLECmCI9cQe6Q9wMSSupqwuRfYZjMPcWfKiqTSlzug2z
    K5i0DaksZczkSabZQY4C2Dhc4IY2WvDZE6CErMmMvWgbC68Uy3fJiyyxYZslA79R
    7tBqNyjZ/uD/3hlxC+tj6W6K01g8pZnftJLqm1PbobPTOzn4OObPfb8rUrWUvN+N
    mICeqNzl9NaOylKo6KtpZrd6w0+AEBhN0O7/3VB2smu/iwCvusSAX0kqiK5r0m6f
    M1H3ksI6jzuHbl4DzhiOGyUpPKczHsG9KWid58Z3/JWl86J4jE1yt8zdAP7fk+dO
    4OET19pMmMHYg9NKRW1HpQhUbx3oOhEv1g==
    -----END CERTIFICATE-----
    
- path: /etc/haproxy/ca.key
  owner: haproxy:haproxy
  permissions: "0440"
  content: |
    -----BEGIN RSA PRIVATE KEY-----
    MIIEpAIBAAKCAQEA4hZ970vuc4xwE12ZZa+3JUH+cqtW6NU/3I5qGz10blq9atIq
    npNmpCofJC6fPY/WP/d5Q0FWwHuraJpzuW8RqzyLyC8zpydmMLKw+6c6lwdO2Znt
    ySVbbEmT/ruDsJ16swdQWUQpnYaDR0rLUYdPQrrTEGlttyvtQsdsCxJZ+lCqvcig
    slUjPRUiTJmgUNzwyrzJYXFMWl0zne0hDm1ElsWldWrUUpHRCcx9KCmXcFcd/2Gq
    iITC23Q0tpNryiPfeB8B30l2QnJAYzGKdEU0YhOVLrcjet0CvYRtNnqlXVecZs7B
    9CMMw8Hh5Hssf3sgX6euk/Kn4h3PTCEYysUBNQIDAQABAoIBAQCNtbdd5GQjvOUK
    3mIl4IuVKNZKHact7WxH3GQZit2NxgZwDCd2mcF+KIC4dxiMx7ltArrZMv0jTODV
    geoDUuDqSdr7sMpZfVKKN5bDRcBtpcEAl4D50RaKu1uuEO6sJykfSfhM23KSMBvc
    9b6W7Y76rotZABwq8beXYdQQ5IHNaNDTvNvrmtOpVnzHjwcw0ZWMLqwipcmuhLot
    vm9RriOcIJqAT13LAkp4+29fpP5RMTmNseTYZryN9UKg1BToeP8jVALxgx48yxLE
    tYVHN3TIq5pNk7D1CJ/HAcMhxEL3D5St0RGCv7GzGWC5+Wk486V/tnhpvKIJPI+5
    nShBsuxBAoGBAPXDP9/8qDVOFuhwdo9a+T3rOCN2iXNlAndJgCLdv8Kf1u92o8hu
    mKLGvnSbnt67zVWIZWSSmstCn8zSYNj96BkCmHiIXrhAeeBQPJsf60kMBnrYgs/Z
    gOqzH9JOw1NcnIkyhZVSjInn39rWFiKTWI7CRyeRdC4IMxQU/vQLw+JNAoGBAOuB
    b12UcQtF5EzK5nxweA55IwCff5VTiwRoHX2FyTUSahn9K6S8P/FN+VQMu5fsauek
    60gHHfLuXAYijQ1Km4bdgZyJhVLN+WzNHmDJGQsuXPRJ+idHTy0R39MCSxcNSonQ
    +TisMyg3EEIKVJ24J4OlgPusH7NPv6yhm85tS36JAoGBAOLb6QqJ33vVKbBGoCqU
    f554ksmpkhfDFhOm9XE54Nl3UqCZk3ZhIOShMQ3S2UQhd9mMnovICLu4NGqNiHjF
    aIotqzEYMNdELTyy1D8dp8M2JoUfdyEGVcpQrv8jVYqN4rGCwWylVrW2JR2MocIo
    4YZmL+iGjAgx6XSQLQh6E8fBAoGAbQW/i1/DsUdKt+4aIzNhsLmNZaVwx60kJwcX
    19sOWV5L9foIsTtgkpHZQXqfgWY120SykuaQi7yip0hpaeTG+PkkHlZffQTTWfXf
    AUk3KcDt0T1J69MMKT4kEqf2IRbLEd/G7+Bv0kcjZJ8pqtXsnPoKKvf0uOrLPdyW
    p0pbb5kCgYBTs+/RbqdeTpbkKXBcaYYcIpr9+HOiaGLl1LHhuOPzU8AqV5mOXg4d
    jidtLwlOklv6wUvMVJMchUjJJHpY9kn6E++3QvB29BcfK85fP9oKf0WB0Lp0z13G
    t+d3Y+TAco6lzgo1lIUn/LxxPodeLiCN04saLy7jGlUmWQF53lSN2Q==
    -----END RSA PRIVATE KEY-----
    

runcmd:
- "hostname \"{{ ds.meta_data.hostname }}\""
- "hostnamectl set-hostname \"{{ ds.meta_data.hostname }}\""
- "echo \"::1         ipv6-localhost ipv6-loopback\" >/etc/hosts"
- "echo \"127.0.0.1   localhost {{ ds.meta_data.hostname }}\" >>/etc/hosts"
- "echo \"127.0.0.1   {{ ds.meta_data.hostname }}\" >>/etc/hosts"
- "echo \"{{ ds.meta_data.hostname }}\" >/etc/hostname"
- "new-cert.sh -1 /etc/haproxy/ca.crt -2 /etc/haproxy/ca.key -3 \"127.0.0.1,{{ ds.meta_data.local_ipv4 }}\" -4 \"localhost\" \"{{ ds.meta_data.hostname }}\" /etc/haproxy"
users:
- name: capv
  sudo: ALL=(ALL) NOPASSWD:ALL
  ssh_authorized_keys:
  - "publicKey"
`
)
