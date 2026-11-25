package label

const TestGTIN = "04600000000015"

const TestSerial = "TEST0000001"

func TestZPL() []byte {
	return []byte("^XA\n" +
		"^PW400\n" +
		"^LL300\n" +
		"^LH0,0\n" +
		"^FO20,20^A0N,30,30^FDMarkirovka agent^FS\n" +
		"^FO20,60^A0N,24,24^FDTest print OK^FS\n" +
		"^FO20,95^A0N,20,20^FD(01)" + TestGTIN + "^FS\n" +
		"^FO20,120^A0N,20,20^FD(21)" + TestSerial + "^FS\n" +
		"^FO240,95^BXN,5,200,0,0,1,~^FD~101" + TestGTIN + "21" + TestSerial + "~193TEST^FS\n" +
		"^XZ\n")
}
