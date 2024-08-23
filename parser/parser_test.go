package parser

import (
	"fmt"
	"testing"
)

func TestEncodeString(t *testing.T) {
	// Example usage
	//decoder := Decoder{}
	pkt := Packet{
		Type: EVENT,
		Nsp:  "/chat",
		Data: map[string]interface{}{
			"message": "Hello, World!",
		},
	}

	encodedPackets := encodeAsString(pkt)
	fmt.Println("Encoded packets:", encodedPackets)
}

func TestDecoderString(t *testing.T) {
	pkg, _ := DecodeString(`2`)
	fmt.Println("DecodeString packets:", pkg)

	// pkg, _ = DecodeString(`2/chat,`)
	// fmt.Println("DecodeString packets:", pkg)

	// pkg, _ = DecodeString(`2/chat,123`)
	// fmt.Println("DecodeString packets:", pkg)
	// fmt.Println("DecodeString id:", *pkg.Id)

	// pkg, _ = DecodeString(`2{"message":"Hello, World!"}`)
	// fmt.Println("DecodeString packets:", pkg)

	pkg, _ = DecodeString(`2/chat,123{"message":"Hello, World!"}`)
	fmt.Println("DecodeString packets:", pkg)

	// pkg, _ = DecodeString(`51-123{"message":"Hello, World!"}`)
	// fmt.Println("DecodeString packets:", pkg)

	pkg, _ = DecodeString(`51-/chat,123{"message":"Hello, World!"}`)
	fmt.Println("DecodeString packets:", pkg)
}
