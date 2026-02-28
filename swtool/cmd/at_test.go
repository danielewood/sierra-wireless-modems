package cmd

import "testing"

func TestIsDangerousAT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cmd       string
		dangerous bool
	}{
		// Safe read-only commands
		{name: "basic AT", cmd: "AT", dangerous: false},
		{name: "ATI query", cmd: "ATI", dangerous: false},
		{name: "band query", cmd: "AT!BAND?", dangerous: false},
		{name: "selrat query", cmd: "AT!SELRAT?", dangerous: false},
		{name: "usbcomp query", cmd: "AT!USBCOMP?", dangerous: false},
		{name: "signal query", cmd: "AT+CSQ", dangerous: false},
		{name: "gstatus query", cmd: "AT!GSTATUS?", dangerous: false},
		{name: "impref query", cmd: "AT!IMPREF?", dangerous: false},
		{name: "priid query", cmd: "AT!PRIID?", dangerous: false},
		{name: "custom query", cmd: "AT!CUSTOM?", dangerous: false},
		{name: "image query", cmd: "AT!IMAGE?", dangerous: false},
		{name: "cfun query", cmd: "AT+CFUN?", dangerous: false},

		// Safe syntax query forms (=?)
		{name: "band syntax", cmd: "AT!BAND=?", dangerous: false},
		{name: "selrat syntax", cmd: "AT!SELRAT=?", dangerous: false},
		{name: "usbcomp syntax", cmd: "AT!USBCOMP=?", dangerous: false},
		{name: "usbspeed syntax", cmd: "AT!USBSPEED=?", dangerous: false},
		{name: "cfun syntax", cmd: "AT+CFUN=?", dangerous: false},

		// Dangerous mutation commands
		{name: "reset", cmd: "AT!RESET", dangerous: true},
		{name: "image clear", cmd: "AT!IMAGE=0", dangerous: true},
		{name: "usbcomp set", cmd: "AT!USBCOMP=1,1,0000100D", dangerous: true},
		{name: "usbvid set", cmd: "AT!USBVID=1199", dangerous: true},
		{name: "usbpid set", cmd: "AT!USBPID=9071,9070", dangerous: true},
		{name: "usbproduct set", cmd: "AT!USBPRODUCT=\"EM7455\"", dangerous: true},
		{name: "usbspeed set", cmd: "AT!USBSPEED=1", dangerous: true},
		{name: "selrat set", cmd: "AT!SELRAT=06", dangerous: true},
		{name: "band set", cmd: "AT!BAND=09", dangerous: true},
		{name: "impref set", cmd: "AT!IMPREF=\"GENERIC\"", dangerous: true},
		{name: "gobiimpref set", cmd: "AT!GOBIIMPREF=\"GENERIC\"", dangerous: true},
		{name: "priid set", cmd: "AT!PRIID=\"9999999\",\"001.001\",\"Generic\"", dangerous: true},
		{name: "custom set", cmd: "AT!CUSTOM=\"FASTENUMEN\",2", dangerous: true},
		{name: "pcoffen set", cmd: "AT!PCOFFEN=2", dangerous: true},
		{name: "entercnd set", cmd: "AT!ENTERCND=\"A710\"", dangerous: true},
		{name: "cfun set", cmd: "AT+CFUN=0", dangerous: true},
		{name: "cfun reset", cmd: "AT+CFUN=1", dangerous: true},
		{name: "factory reset", cmd: "AT&F", dangerous: true},
		{name: "boothold", cmd: "AT!BOOTHOLD", dangerous: true},
		{name: "lteca disable", cmd: "AT!LTECA=0", dangerous: true},
		{name: "lteca enable", cmd: "AT!LTECA=1", dangerous: true},
		{name: "gpsfix", cmd: "AT!GPSFIX=1,30,10", dangerous: true},
		{name: "rma reset", cmd: "AT!RMARESET=1", dangerous: true},
		{name: "nv restore", cmd: "AT!NVRESTORE=0", dangerous: true},
		{name: "set apn", cmd: "AT+CGDCONT=1,\"IP\",\"internet\"", dangerous: true},
		{name: "send sms", cmd: "AT+CMGS=\"+15551234567\"", dangerous: true},
		{name: "airvantage interval", cmd: "AT+WDSC=3,60", dangerous: true},
		{name: "airvantage heartbeat", cmd: "AT+WDSS=1,1", dangerous: true},

		// GPS/LTE queries are safe
		{name: "gps location query", cmd: "AT!GPSLOC?", dangerous: false},
		{name: "gps status query", cmd: "AT!GPSSTATUS?", dangerous: false},
		{name: "gpsfix syntax", cmd: "AT!GPSFIX=?", dangerous: false},
		{name: "lteca query", cmd: "AT!LTECA?", dangerous: false},
		{name: "cgdcont query", cmd: "AT+CGDCONT?", dangerous: false},

		// Case insensitivity
		{name: "lowercase reset", cmd: "at!reset", dangerous: true},
		{name: "mixed case band", cmd: "At!Band=09", dangerous: true},
		{name: "lowercase query", cmd: "at!band?", dangerous: false},

		// Whitespace handling
		{name: "leading space", cmd: "  AT!RESET", dangerous: true},
		{name: "trailing space", cmd: "AT!BAND?  ", dangerous: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isDangerousAT(tt.cmd)
			if got != tt.dangerous {
				t.Errorf("isDangerousAT(%q) = %v, want %v", tt.cmd, got, tt.dangerous)
			}
		})
	}
}
