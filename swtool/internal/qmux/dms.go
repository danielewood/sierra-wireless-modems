package qmux

import "fmt"

// DMS message IDs.
const (
	dmsMsgGetOperatingMode    uint16 = 0x002D
	dmsMsgSetOperatingMode    uint16 = 0x002E
	dmsMsgGetFirmwarePref     uint16 = 0x0047
	dmsMsgSetFirmwarePref     uint16 = 0x0048
	dmsMsgListStoredImages    uint16 = 0x0049
	dmsMsgDeleteStoredImage   uint16 = 0x004A
	dmsSWISetUSBComposition   uint16 = 0x555C // Sierra vendor extension
)

// Operating mode constants for DMS Get/Set Operating Mode.
const (
	ModeOnline   uint8 = 0x00
	ModeLowPower uint8 = 0x01
	ModeOffline  uint8 = 0x03
	ModeReset    uint8 = 0x04
	ModeShutdown uint8 = 0x05
)

// ModeString returns a human-readable name for an operating mode value.
func ModeString(mode uint8) string {
	switch mode {
	case ModeOnline:
		return "online"
	case ModeLowPower:
		return "low-power"
	case ModeOffline:
		return "offline"
	case ModeReset:
		return "reset"
	case ModeShutdown:
		return "shutting-down"
	default:
		return fmt.Sprintf("unknown-%d", mode)
	}
}

// GetOperatingMode queries the modem's DMS operating mode.
func GetOperatingMode(c *Conn) (uint8, error) {
	resp, err := c.Send(ServiceDMS, dmsMsgGetOperatingMode)
	if err != nil {
		return 0, fmt.Errorf("getting operating mode: %w", err)
	}
	if err := resp.Result(); err != nil {
		return 0, fmt.Errorf("getting operating mode: %w", err)
	}
	tlv := resp.FindTLV(0x01)
	if tlv == nil {
		return 0, fmt.Errorf("getting operating mode: missing mode TLV")
	}
	return DecodeTLVU8(tlv)
}

// SetOperatingMode sets the modem's DMS operating mode.
func SetOperatingMode(c *Conn, mode uint8) error {
	resp, err := c.Send(ServiceDMS, dmsMsgSetOperatingMode, EncodeTLVU8(0x01, mode))
	if err != nil {
		return fmt.Errorf("setting operating mode: %w", err)
	}
	if err := resp.Result(); err != nil {
		return fmt.Errorf("setting operating mode: %w", err)
	}
	return nil
}

// StoredImage represents a firmware image slot from DMS ListStoredImages.
type StoredImage struct {
	Type         uint8  // 0 = modem, 1 = pri
	Slot         uint8  // index within type
	UniqueID     string
	BuildID      string
	StorageIndex uint8
	FailureCount uint8
}

// ListStoredImages queries the modem's stored firmware images.
//
// DMS 0x0049 response TLV 0x01 layout (nested, variable-length):
//
//	count(1)
//	  per list entry:
//	    type(1) maxImages(1) indexListSize(1)
//	      per image in list:
//	        storageIndex(1) failureCount(1)
//	  imageIDListSize(1)
//	      per image ID:
//	        uniqueIDLen(1) uniqueID(N) buildIDLen(1) buildID(N)
func ListStoredImages(c *Conn) ([]StoredImage, error) {
	resp, err := c.Send(ServiceDMS, dmsMsgListStoredImages)
	if err != nil {
		return nil, fmt.Errorf("listing stored images: %w", err)
	}
	if err := resp.Result(); err != nil {
		return nil, fmt.Errorf("listing stored images: %w", err)
	}

	tlv := resp.FindTLV(0x01)
	if tlv == nil {
		return nil, fmt.Errorf("listing stored images: missing image list TLV")
	}

	return parseStoredImagesTLV(tlv.Value)
}

func parseStoredImagesTLV(data []byte) ([]StoredImage, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("stored images TLV too short")
	}

	var images []StoredImage
	off := 0
	listCount := int(data[off])
	off++

	for i := range listCount {
		if off+3 > len(data) {
			return nil, fmt.Errorf("truncated list entry %d header", i)
		}
		imageType := data[off]
		off++ // type
		off++ // maxImages (skip)
		indexListSize := int(data[off])
		off++

		// Read index list entries (storageIndex + failureCount per entry).
		type indexEntry struct {
			storageIndex uint8
			failureCount uint8
		}
		indices := make([]indexEntry, indexListSize)
		for j := range indexListSize {
			if off+2 > len(data) {
				return nil, fmt.Errorf("truncated index entry %d/%d", i, j)
			}
			indices[j] = indexEntry{
				storageIndex: data[off],
				failureCount: data[off+1],
			}
			off += 2
		}

		// Read image ID list.
		if off+1 > len(data) {
			return nil, fmt.Errorf("truncated image ID list size for entry %d", i)
		}
		idListSize := int(data[off])
		off++

		for j := range idListSize {
			if off+1 > len(data) {
				return nil, fmt.Errorf("truncated unique ID length for entry %d/%d", i, j)
			}
			uidLen := int(data[off])
			off++
			if off+uidLen > len(data) {
				return nil, fmt.Errorf("truncated unique ID for entry %d/%d", i, j)
			}
			uniqueID := string(data[off : off+uidLen])
			off += uidLen

			if off+1 > len(data) {
				return nil, fmt.Errorf("truncated build ID length for entry %d/%d", i, j)
			}
			bidLen := int(data[off])
			off++
			if off+bidLen > len(data) {
				return nil, fmt.Errorf("truncated build ID for entry %d/%d", i, j)
			}
			buildID := string(data[off : off+bidLen])
			off += bidLen

			img := StoredImage{
				Type:     imageType,
				Slot:     uint8(j),
				UniqueID: uniqueID,
				BuildID:  buildID,
			}
			if j < len(indices) {
				img.StorageIndex = indices[j].storageIndex
				img.FailureCount = indices[j].failureCount
			}
			images = append(images, img)
		}
	}

	return images, nil
}

// FirmwarePrefImage represents a firmware preference entry.
type FirmwarePrefImage struct {
	Type     uint8 // 0 = modem, 1 = pri
	UniqueID string
	BuildID  string
}

// GetFirmwarePreference queries the current firmware preference.
//
// DMS 0x0047 response TLV 0x01 layout:
//
//	count(1)
//	  per entry: type(1) uniqueIDLen(1) uniqueID(N) buildIDLen(1) buildID(N)
func GetFirmwarePreference(c *Conn) ([]FirmwarePrefImage, error) {
	resp, err := c.Send(ServiceDMS, dmsMsgGetFirmwarePref)
	if err != nil {
		return nil, fmt.Errorf("getting firmware preference: %w", err)
	}
	if err := resp.Result(); err != nil {
		return nil, fmt.Errorf("getting firmware preference: %w", err)
	}

	tlv := resp.FindTLV(0x01)
	if tlv == nil {
		return nil, fmt.Errorf("getting firmware preference: missing preference TLV")
	}

	return parseFirmwarePrefTLV(tlv.Value)
}

func parseFirmwarePrefTLV(data []byte) ([]FirmwarePrefImage, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("firmware preference TLV too short")
	}

	off := 0
	count := int(data[off])
	off++

	images := make([]FirmwarePrefImage, 0, count)
	for i := range count {
		if off+1 > len(data) {
			return nil, fmt.Errorf("truncated firmware preference entry %d", i)
		}
		imgType := data[off]
		off++

		if off+1 > len(data) {
			return nil, fmt.Errorf("truncated unique ID length for preference %d", i)
		}
		uidLen := int(data[off])
		off++
		if off+uidLen > len(data) {
			return nil, fmt.Errorf("truncated unique ID for preference %d", i)
		}
		uniqueID := string(data[off : off+uidLen])
		off += uidLen

		if off+1 > len(data) {
			return nil, fmt.Errorf("truncated build ID length for preference %d", i)
		}
		bidLen := int(data[off])
		off++
		if off+bidLen > len(data) {
			return nil, fmt.Errorf("truncated build ID for preference %d", i)
		}
		buildID := string(data[off : off+bidLen])
		off += bidLen

		images = append(images, FirmwarePrefImage{
			Type:     imgType,
			UniqueID: uniqueID,
			BuildID:  buildID,
		})
	}

	return images, nil
}

// SetFirmwarePreference sets the modem's firmware preference.
// fwVersion, configVersion, and carrier are used to build a matching
// modem + PRI preference pair.
func SetFirmwarePreference(c *Conn, fwVersion, configVersion, carrier string) error {
	// Build preference TLV 0x01: count(1) + 2 entries (modem + PRI).
	modemBuildID := fwVersion + "_" + carrier
	priBuildID := modemBuildID

	// Modem entry: type=0, uniqueID="?_?", buildID="{fw}_{carrier}"
	// PRI entry: type=1, uniqueID=configVersion, buildID="{fw}_{carrier}"
	var payload []byte
	payload = append(payload, 0x02) // count = 2

	// Modem entry.
	payload = append(payload, 0x00)                     // type = modem
	payload = append(payload, byte(len("?_?")))          // uniqueID length
	payload = append(payload, []byte("?_?")...)          // uniqueID
	payload = append(payload, byte(len(modemBuildID)))   // buildID length
	payload = append(payload, []byte(modemBuildID)...)   // buildID

	// PRI entry.
	payload = append(payload, 0x01)                      // type = pri
	payload = append(payload, byte(len(configVersion)))  // uniqueID length
	payload = append(payload, []byte(configVersion)...)  // uniqueID
	payload = append(payload, byte(len(priBuildID)))     // buildID length
	payload = append(payload, []byte(priBuildID)...)     // buildID

	resp, err := c.Send(ServiceDMS, dmsMsgSetFirmwarePref, TLV{Type: 0x01, Value: payload})
	if err != nil {
		return fmt.Errorf("setting firmware preference: %w", err)
	}
	if err := resp.Result(); err != nil {
		return fmt.Errorf("setting firmware preference: %w", err)
	}
	return nil
}

// SelectStoredImage selects active firmware image slots.
// modemSlot and priSlot are 0-based indices.
//
// This is implemented as a SetFirmwarePreference with the build IDs from
// the selected slots. There is no dedicated "select" message in QMI;
// qmicli's --dms-select-stored-image internally sets firmware preference.
func SelectStoredImage(c *Conn, images []StoredImage, modemSlot, priSlot int) error {
	var modemImg, priImg *StoredImage
	for i := range images {
		if images[i].Type == 0 && int(images[i].Slot) == modemSlot {
			modemImg = &images[i]
		}
		if images[i].Type == 1 && int(images[i].Slot) == priSlot {
			priImg = &images[i]
		}
	}
	if modemImg == nil || priImg == nil {
		return fmt.Errorf("selecting stored image: slot modem=%d pri=%d not found", modemSlot, priSlot)
	}

	var payload []byte
	payload = append(payload, 0x02) // count = 2

	// Modem entry.
	payload = append(payload, 0x00)
	payload = append(payload, byte(len(modemImg.UniqueID)))
	payload = append(payload, []byte(modemImg.UniqueID)...)
	payload = append(payload, byte(len(modemImg.BuildID)))
	payload = append(payload, []byte(modemImg.BuildID)...)

	// PRI entry.
	payload = append(payload, 0x01)
	payload = append(payload, byte(len(priImg.UniqueID)))
	payload = append(payload, []byte(priImg.UniqueID)...)
	payload = append(payload, byte(len(priImg.BuildID)))
	payload = append(payload, []byte(priImg.BuildID)...)

	resp, err := c.Send(ServiceDMS, dmsMsgSetFirmwarePref, TLV{Type: 0x01, Value: payload})
	if err != nil {
		return fmt.Errorf("selecting stored image: %w", err)
	}
	if err := resp.Result(); err != nil {
		return fmt.Errorf("selecting stored image: %w", err)
	}
	return nil
}

// DeleteStoredImage deletes a stored firmware image.
//
// DMS 0x004A request TLV 0x01 layout:
//
//	type(1) uniqueIDLen(1) uniqueID(N) buildIDLen(1) buildID(N)
func DeleteStoredImage(c *Conn, img StoredImage) error {
	var payload []byte
	payload = append(payload, img.Type)
	payload = append(payload, byte(len(img.UniqueID)))
	payload = append(payload, []byte(img.UniqueID)...)
	payload = append(payload, byte(len(img.BuildID)))
	payload = append(payload, []byte(img.BuildID)...)

	resp, err := c.Send(ServiceDMS, dmsMsgDeleteStoredImage, TLV{Type: 0x01, Value: payload})
	if err != nil {
		return fmt.Errorf("deleting stored image: %w", err)
	}
	if err := resp.Result(); err != nil {
		return fmt.Errorf("deleting stored image: %w", err)
	}
	return nil
}

// SetUSBComposition sets the Sierra USB composition via vendor DMS extension.
//
// DMS 0x555C request TLV 0x01: compositionID(4, little-endian)
func SetUSBComposition(c *Conn, compositionID uint8) error {
	// Sierra's vendor extension expects a uint32 for the composition ID.
	resp, err := c.Send(ServiceDMS, dmsSWISetUSBComposition, EncodeTLVU32(0x01, uint32(compositionID)))
	if err != nil {
		return fmt.Errorf("setting USB composition: %w", err)
	}
	if err := resp.Result(); err != nil {
		return fmt.Errorf("setting USB composition: %w", err)
	}
	return nil
}

// ImageTypeString returns "modem" or "pri" for the QMI image type byte.
func ImageTypeString(t uint8) string {
	switch t {
	case 0:
		return "modem"
	case 1:
		return "pri"
	default:
		return fmt.Sprintf("unknown-%d", t)
	}
}

// ImageTypeFromString returns the QMI image type byte for "modem" or "pri".
func ImageTypeFromString(s string) (uint8, error) {
	switch s {
	case "modem":
		return 0, nil
	case "pri":
		return 1, nil
	default:
		return 0, fmt.Errorf("unknown image type: %q", s)
	}
}

// CurrentImages returns the subset of images that the modem considers
// "current" based on the firmware preference. The modem marks current
// images via the response's optional TLV 0x10 (current list).
// For simplicity, we pass through the preference-based current info
// at the qmi/ layer.

// encodeFirmwarePrefPayload builds the TLV 0x01 value for
// SetFirmwarePreference / SelectStoredImage from a list of images.
func encodeFirmwarePrefPayload(images []FirmwarePrefImage) []byte {
	var payload []byte
	payload = append(payload, byte(len(images)))
	for _, img := range images {
		payload = append(payload, img.Type)
		payload = append(payload, byte(len(img.UniqueID)))
		payload = append(payload, []byte(img.UniqueID)...)
		payload = append(payload, byte(len(img.BuildID)))
		payload = append(payload, []byte(img.BuildID)...)
	}
	return payload
}

// buildIDParts splits a build ID like "02.39.00.00_GENERIC" into
// (firmware version, carrier). Uses last underscore as delimiter.
func buildIDParts(buildID string) (fwVersion, carrier string) {
	for i := len(buildID) - 1; i >= 0; i-- {
		if buildID[i] == '_' {
			return buildID[:i], buildID[i+1:]
		}
	}
	return buildID, ""
}

// GetCurrentImageInfo returns the current firmware preference images,
// parsed into a pair of (modem image, pri image). Returns nil for
// either if not present.
func GetCurrentImageInfo(c *Conn) (modem *FirmwarePrefImage, pri *FirmwarePrefImage, err error) {
	images, err := GetFirmwarePreference(c)
	if err != nil {
		return nil, nil, err
	}
	for i := range images {
		switch images[i].Type {
		case 0:
			modem = &images[i]
		case 1:
			pri = &images[i]
		}
	}
	return modem, pri, nil
}

// ListStoredImagesWithCurrent queries stored images and marks which ones
// match the current firmware preference. The QMI protocol doesn't have
// a single query that returns both; we need to combine ListStoredImages
// with GetFirmwarePreference.
type StoredImageWithCurrent struct {
	StoredImage
	Current bool
}

// ListStoredImagesEx queries stored images and marks which ones are current
// based on the firmware preference.
func ListStoredImagesEx(c *Conn) ([]StoredImageWithCurrent, error) {
	images, err := ListStoredImages(c)
	if err != nil {
		return nil, err
	}

	prefs, prefErr := GetFirmwarePreference(c)

	result := make([]StoredImageWithCurrent, len(images))
	for i, img := range images {
		result[i] = StoredImageWithCurrent{StoredImage: img}
		if prefErr == nil {
			for _, pref := range prefs {
				if pref.Type == img.Type && pref.UniqueID == img.UniqueID && pref.BuildID == img.BuildID {
					result[i].Current = true
					break
				}
			}
		}
	}
	return result, nil
}
