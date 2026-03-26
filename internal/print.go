package domain

import "fmt"

// MostrarDatos imprime todos los datos almacenados
func (a *DataBet) PrintData() {

	// si imprimiese durante la ejecuccion y no al final
	a.mu.Lock()
	defer a.mu.Unlock()

	fmt.Println("DataBet Matches:")
	for match, betMatch := range a.Matches {
		fmt.Println("Match:", match)

		betMatch.mu.Lock()
		// Para cada `BetMatch`, verificamos sus tipos de apuestas
		for name, dataBetType := range map[string]*DataBetType{
			"BTG": betMatch.BTG,
			"TPI": betMatch.TPI,
			"TUO": betMatch.TUO,
			"FUO": betMatch.FUO,
		} {
			if dataBetType == nil {
				continue
			}

			dataBetType.mu.Lock()
			fmt.Printf("  %s:\n", name)
			for house, idents := range dataBetType.data { // Casa de apuestas
				fmt.Printf("    Casa: %s\n", house)
				for ident, headers := range idents { // Identificadores
					fmt.Printf("      Ident: %s\n", ident)
					for header, odd := range headers { // Headers y cuotas
						fmt.Printf("        Header: %s, Cuota: %.3f\n", header, odd)
					}
				}
			}
			dataBetType.mu.Unlock()
		}
		betMatch.mu.Unlock()
	}
}
