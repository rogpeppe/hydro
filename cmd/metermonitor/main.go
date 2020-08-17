// +build ignore

package main

type Config struct {
	ListenAddr string
	StateDir   string
}


	listen address
	meter address
	sample directory
	ntp server address (pool.ntp.org)

	start sampler
	start http server

	sampler:
		get current time from ntp server
		create file with name from current time
		forever {
			get sample
			append sample to file
		}
