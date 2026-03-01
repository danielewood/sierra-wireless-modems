// Package modem provides detection, serial communication, and AT command
// handling for Sierra Wireless EM7455/MC7455/EM7565 modems.
package modem

// USBID represents a vendor:product ID pair.
type USBID struct {
	Vendor  string
	Product string
}

// String returns the VID:PID format used by qmi-firmware-update.
func (id USBID) String() string {
	return id.Vendor + ":" + id.Product
}

// Device holds all discovered paths and identity for a detected modem.
type Device struct {
	ID          USBID  // e.g. {413c, 81b6}
	Name        string // Human name, e.g. "Dell DW5811e"
	SysfsPath   string // e.g. "/sys/bus/usb/devices/1-3"
	ATPort      string // e.g. "/dev/ttyUSB2"
	CDCDevice   string // e.g. "/dev/cdc-wdm0" (MBIM or QMI via qmi_wwan)
	QCQMIDevice string // e.g. "/dev/qcqmi0" (QMI via GobiNet)
	Bootloader  bool   // true when modem is in QDL/bootloader mode
}

// OnlineIDs maps known VID:PID pairs for modems in normal operating mode.
var OnlineIDs = map[USBID]string{
	{Vendor: "1199", Product: "9071"}: "Sierra Wireless EM7455",
	{Vendor: "1199", Product: "9079"}: "Lenovo EM7455",
	{Vendor: "413c", Product: "81b6"}: "Dell DW5811e",
}

// BootloaderIDs maps known VID:PID pairs for modems in bootloader/QDL mode.
var BootloaderIDs = map[USBID]string{
	{Vendor: "1199", Product: "9070"}: "Sierra Wireless EM7455 (bootloader)",
	{Vendor: "1199", Product: "9078"}: "Lenovo EM7455 (bootloader)",
	{Vendor: "413c", Product: "81b5"}: "Dell DW5811e (bootloader)",
}

// USBComposition represents a USB interface composition mode.
type USBComposition struct {
	ATValue     string // AT!USBCOMP value, e.g. "1,1,0000100D"
	QMICLIValue string // qmicli --dms-swi-set-usb-composition value, e.g. "8"
	Description string // Human-readable description
}

var (
	// CompMBIM is MBIM mode: diag + nmea + modem + mbim.
	CompMBIM = USBComposition{
		ATValue:     "1,1,0000100D",
		QMICLIValue: "8",
		Description: "MBIM (diag, nmea, modem, mbim)",
	}

	// CompQMI is QMI mode: diag + nmea + modem + rmnet0.
	CompQMI = USBComposition{
		ATValue:     "1,1,0000010D",
		QMICLIValue: "6",
		Description: "QMI (diag, nmea, modem, rmnet0)",
	}
)

// VendorProfile holds USB identity values for a modem vendor.
type VendorProfile struct {
	VID     string // AT!USBVID value, e.g. "1199"
	PIDApp  string // Application mode PID, e.g. "9071"
	PIDBoot string // Bootloader mode PID, e.g. "9070"
	Product string // AT!USBPRODUCT value, e.g. "EM7455"
}

// Vendors maps vendor names to their USB identity profiles.
var Vendors = map[string]VendorProfile{
	"sierra": {VID: "1199", PIDApp: "9071", PIDBoot: "9070", Product: "EM7455"},
	"dell":   {VID: "413C", PIDApp: "81B6", PIDBoot: "81B5", Product: "DW5811e Snapdragon\u2122 X7 LTE"},
	"lenovo": {VID: "1199", PIDApp: "9079", PIDBoot: "9078", Product: "Sierra Wireless EM7455 Qualcomm Snapdragon X7 LTE-A"},
}

// CommonATCommands lists frequently used AT commands for tab completion.
var CommonATCommands = []string{
	// Basic
	"AT",
	"ATI",
	"AT!ENTERCND=\"A710\"",
	// Firmware & carrier
	"AT!IMPREF?",
	"AT!GOBIIMPREF?",
	"AT!IMAGE?",
	"AT!PRIID?",
	// USB configuration
	"AT!USBCOMP?",
	"AT!USBCOMP=?",
	"AT!USBVID?",
	"AT!USBPID?",
	"AT!USBPRODUCT?",
	"AT!USBSPEED?",
	"AT!USBSPEED=?",
	// Network & signal
	"AT!SELRAT?",
	"AT!SELRAT=?",
	"AT!BAND?",
	"AT!BAND=?",
	"AT!GSTATUS?",
	"AT!LTEINFO?",
	"AT!LTECA?",
	"AT+CSQ",
	"AT+COPS?",
	"AT+COPS=?",
	"AT+CREG?",
	"AT+CEREG?",
	"AT+CGREG?",
	// Power control
	"AT!PCINFO?",
	"AT!PCOFFEN?",
	"AT!CUSTOM?",
	// SIM
	"AT+CPIN?",
	"AT+CIMI",
	"AT+ICCID?",
	// Diagnostics
	"AT!GBAND?",
	"AT!LTEINFO=SERVING",
	"AT!LTEINFO=INTRA",
	"AT!LTEINFO=INTER",
	// Control
	"AT!RESET",
	"AT+CFUN=0",
	"AT+CFUN=1",
}
