package hydroctl

// PowerChargeable holds power as it will be allocated to
// chargeable units.
type PowerChargeable struct {
	ExportGrid      float64
	ExportNeighbour float64
	ExportHere      float64
	ImportNeighbour float64
	ImportHere      float64
}

// PowerUse holds how power is being
// used and generated in the system.
type PowerUse struct {
	Generated float64
	Neighbour float64
	Here      float64
}

// ChargeablePower calculates how power use will be charged.
func ChargeablePower(pu PowerUse) PowerChargeable {
	halfPower := pu.Generated / 2
	imported := (pu.Neighbour + pu.Here) - pu.Generated
	switch {
	case imported <= 0:
		// Between us we're using less than the amount we're generating, so
		// it's all at export rates.
		return PowerChargeable{
			ExportNeighbour: pu.Neighbour,
			ExportHere:      pu.Here,
			ExportGrid:      pu.Generated - (pu.Neighbour + pu.Here),
		}
	case pu.Neighbour > halfPower && pu.Here > halfPower:
		// Both of us are using more than half the available power - allocate
		// the power proportionally.
		neighbourRatio := pu.Neighbour / (pu.Neighbour + pu.Here)
		return PowerChargeable{
			ExportNeighbour: halfPower,
			ExportHere:      halfPower,
			ImportNeighbour: neighbourRatio * imported,
			ImportHere:      (1 - neighbourRatio) * imported,
		}
	case pu.Neighbour > halfPower:
		// Only our neighbour is using more than half the power, so
		// they get any available generated power before importing.
		return PowerChargeable{
			ExportNeighbour: pu.Generated - pu.Here,
			ExportHere:      pu.Here,
			ImportNeighbour: imported,
		}
	case pu.Here > halfPower:
		// Only here is using more than half the power, so
		// we get any available generated power before importing.
		return PowerChargeable{
			ExportNeighbour: pu.Neighbour,
			ExportHere:      pu.Generated - pu.Neighbour,
			ImportHere:      imported,
		}
	default:
		panic("unreachable")
	}
}
