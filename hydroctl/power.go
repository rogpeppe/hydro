package hydroctl

// PowerChargeable holds power as it will be allocated to
// chargeable units.
type PowerChargeable struct {
	// ExportGrid holds the power exported to the grid (W).
	ExportGrid float64
	// ExportNeighbour holds the exported power used next door (W).
	ExportNeighbour float64
	// ExportHere holds the exported power used by here (W).
	ExportHere float64
	// ImportNeighbour holds the import power used next door (W).
	ImportNeighbour float64
	// ImportHere holds the import power used here (W).
	ImportHere float64
}

// PowerUse holds how power is being
// used and generated in the system.
type PowerUse struct {
	// Generated holds the power being generated in watts.
	Generated float64
	// Neighbour holds the power being used by our neighbour in watts.
	Neighbour float64
	// Here holds the power being used here in watts.
	Here float64
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
