// generated by stringer -type Cmd; DO NOT EDIT

package eth8020

import "fmt"

const (
	_Cmd_name_0 = "CmdModuleInfo"
	_Cmd_name_1 = "CmdDigitalActiveCmdDigitalInactive"
	_Cmd_name_2 = "CmdDigitalSetOutputsCmdDigitalGetOutputsCmdDigitalGetInputs"
	_Cmd_name_3 = "CmdGetAnalogVoltage"
	_Cmd_name_4 = "CmdASCIITextCommand"
	_Cmd_name_5 = "CmdSerialNumberCmdVoltsCmdLoginCmdUnlockTimeCmdLogout"
)

var (
	_Cmd_index_0 = [...]uint8{0, 13}
	_Cmd_index_1 = [...]uint8{0, 16, 34}
	_Cmd_index_2 = [...]uint8{0, 20, 40, 59}
	_Cmd_index_3 = [...]uint8{0, 19}
	_Cmd_index_4 = [...]uint8{0, 19}
	_Cmd_index_5 = [...]uint8{0, 15, 23, 31, 44, 53}
)

func (i Cmd) String() string {
	switch {
	case i == 16:
		return _Cmd_name_0
	case 32 <= i && i <= 33:
		i -= 32
		return _Cmd_name_1[_Cmd_index_1[i]:_Cmd_index_1[i+1]]
	case 35 <= i && i <= 37:
		i -= 35
		return _Cmd_name_2[_Cmd_index_2[i]:_Cmd_index_2[i+1]]
	case i == 50:
		return _Cmd_name_3
	case i == 58:
		return _Cmd_name_4
	case 119 <= i && i <= 123:
		i -= 119
		return _Cmd_name_5[_Cmd_index_5[i]:_Cmd_index_5[i+1]]
	default:
		return fmt.Sprintf("Cmd(%d)", i)
	}
}
