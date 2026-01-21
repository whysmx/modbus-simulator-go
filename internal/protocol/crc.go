package protocol

// CRC16 calculates CRC-16 (Modbus) checksum
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)

	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 == 1 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}

	return crc
}

// CRC16Bytes returns CRC as [low, high] bytes
func CRC16Bytes(data []byte) [2]byte {
	crc := CRC16(data)
	return [2]byte{byte(crc & 0xFF), byte(crc >> 8)}
}

// ValidateCRC checks if the last 2 bytes are a valid CRC
func ValidateCRC(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	payload := data[:len(data)-2]
	expected := CRC16(payload)
	actual := uint16(data[len(data)-2]) | uint16(data[len(data)-1])<<8
	return expected == actual
}
