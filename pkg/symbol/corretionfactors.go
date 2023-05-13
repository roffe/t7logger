package symbol

func GetCorrectionfactor(name string) string {
	//log.Println(name)
	switch name {
	case "IgnProt.fi_Offset", "Out.X_AccPedal", "Out.fi_Ignition",
		"Out.PWM_BoostCntrl", "In.v_Vehicle", "In.p_AirAmbient":
		return "0.1"
	case "ECMStat.p_Diff", "ECMStat.p_DiffThrot", "In.p_AirBefThrottle", "In.p_AirInlet":
		return "0.001"
	default:
		return "1"
	}
}
