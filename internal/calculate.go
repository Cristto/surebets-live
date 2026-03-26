package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// buscas las cuotas contrarias si/no par/impar, las de goles no
// solo se podria crear una funcion del struct con la estructura principal
func CalculateBTG_TPI(structBetType *DataBetType, matchID string, channelSurebet chan Surebet) {

	// en cada casa puede ir escrito de manera diferente
	oppositePairs := map[string][]string{
		"Sí": {"No"}, "Si": {"No"}, "Yes": {"No"},
		"No":  {"Sí", "Si", "Yes"}, // Si es "No", busca los 3 posibles "Sí"
		"Par": {"Impar"}, "Impar": {"Par"},
	}

	/*
		Casa: BetHouse1
			Ident: Ident1
				Header: Header1, Cuota: 1.750
			Ident: Ident2
				Header: Header2, Cuota: 1.800
		Casa: BetHouse2
			Ident: Ident3
				Header: Header3, Cuota: 1.900
	*/
	for casaA, identMapA := range structBetType.data {
		for identA, headersA := range identMapA {
			// Verificar si hay un opuesto definido para identA
			if opposites, exists := oppositePairs[identA]; exists {
				for _, identB := range opposites {
					for casaB, identMapB := range structBetType.data {
						if casaA != casaB { // Solo buscamos en casas diferentes
							if headersB, exists := identMapB[identB]; exists {
								// Iterar sobre los headers de casaA e intentar emparejar con casaB
								for headerA, cuotaA := range headersA {
									if cuotaA == 0 {
										fmt.Printf("[WARN] Cuota inválida en %s (%s - %s): %.2f\n", casaA, identA, headerA, cuotaA)
										continue
									}
									for headerB, cuotaB := range headersB {
										if cuotaB == 0 {
											fmt.Printf("[WARN] Cuota inválida en %s (%s - %s): %.2f\n", casaB, identB, headerB, cuotaB)
											continue
										}

										// Cálculo de surebet: (1/cuotaA) + (1/cuotaB)
										surebet := (1 / cuotaA) + (1 / cuotaB)
										percentWinning := (1 - surebet) * 100
										if surebet < 1 {
											channelSurebet <- Surebet{
												KeyTeam:        matchID,
												House:          casaA,
												Ident:          identA,
												Header:         headerA,
												Odd:            cuotaA,
												OppositeHouse:  casaB,
												OppositeIdent:  identB,
												OppositeHeader: headerB,
												OppositeOdd:    cuotaB,
												Surebet:        surebet,
												Percent:        percentWinning,
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func CalculateTUO_FUO(structBetType *DataBetType, matchID string, channelSurebet chan Surebet) {

	// Mapear términos equivalentes
	positiveHeaders := map[string]bool{"Mas": true, "encima": true, "más de": true}
	negativeHeaders := map[string]bool{"Menos": true, "debajo": true, "menos de": true}

	for casaA, identMapA := range structBetType.data {
		for identA, headersA := range identMapA {
			// Convertir el identificador a número flotante
			identAFloat, err := strconv.ParseFloat(strings.Replace(identA, ",", ".", 1), 64)
			if err != nil {
				fmt.Printf("[ERROR] Ident inválido en %s: %s\n", casaA, identA)
				continue
			}

			// por cada header y cuota
			for headerA, cuotaA := range headersA {
				if cuotaA == 0 {
					fmt.Printf("[WARN] Cuota inválida en %s (%s - %s): %.2f\n", casaA, identA, headerA, cuotaA)
					continue
				}

				// Determinar si headerA es positivo o negativo
				isHeaderAPositive := positiveHeaders[headerA]
				isHeaderANegative := negativeHeaders[headerA]

				for casaB, identMapB := range structBetType.data {
					if casaA == casaB {
						continue // Evitar comparar dentro de la misma casa
					}

					for identB, headersB := range identMapB {
						identBFloat, err := strconv.ParseFloat(strings.Replace(identB, ",", ".", 1), 64)
						if err != nil {
							fmt.Printf("[ERROR] Ident inválido en %s: %s\n", casaB, identB)
							continue
						}

						for headerB, cuotaB := range headersB {
							if cuotaB == 0 {
								fmt.Printf("[WARN] Cuota inválida en %s (%s - %s): %.2f\n", casaB, identB, headerB, cuotaB)
								continue
							}

							// Determinar si headerB es positivo o negativo
							isHeaderBPositive := positiveHeaders[headerB]
							isHeaderBNegative := negativeHeaders[headerB]

							// Reglas de emparejamiento:
							// Si A es "Mas/Encima/Más de", B debe ser "Menos/Debajo/Menos de" con Y > X
							// Si A es "Menos/Debajo/Menos de", B debe ser "Mas/Encima/Más de" con Y < X
							if (isHeaderAPositive && isHeaderBNegative && identBFloat > identAFloat) ||
								(isHeaderANegative && isHeaderBPositive && identBFloat < identAFloat) {

								// Cálculo de surebet
								surebet := (1 / cuotaA) + (1 / cuotaB)
								percentWinning := (1 - surebet) * 100

								if surebet < 1 {
									fmt.Printf("Surebet encontrada para %s\n %s (%s - %s - %.2f) y %s (%s - %s - %.2f): %.4f\n",
										matchID,
										casaA, identA, headerA, cuotaA,
										casaB, identB, headerB, cuotaB,
										surebet)

									channelSurebet <- Surebet{
										KeyTeam:        matchID,
										House:          casaA,
										Ident:          identA,
										Header:         headerA,
										Odd:            cuotaA,
										OppositeHouse:  casaB,
										OppositeIdent:  identB,
										OppositeHeader: headerB,
										OppositeOdd:    cuotaB,
										Surebet:        surebet,
										Percent:        percentWinning,
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func (bm *BetMatch) CalculateSurebets(structBetType *DataBetType, betType, matchID string, channelSurebet chan Surebet) {

	structBetType.mu.Lock()
	// Verificar si ya está calculando, porque cada 500ms en monitor podria tirar otra gorrutina concurrentemente
	if structBetType.Calculating {
		structBetType.mu.Unlock()
		fmt.Printf("Ya se están calculando los surebets para %s. Esperando...\n", betType)
		return
	}
	structBetType.Calculating = true // Iniciar el cálculo
	structBetType.mu.Unlock()

	// dentro de structBetType ya tengo toda la informacion que necesito de las cuotas

	if betType == "BTG" || betType == "TPI" {
		CalculateBTG_TPI(structBetType, matchID, channelSurebet)
	} else if betType == "TUO" || betType == "FUO" {
		CalculateTUO_FUO(structBetType, matchID, channelSurebet)
	}

	structBetType.mu.Lock()
	structBetType.Calculating = false
	// actualizamos LastUpdate para que Monitor... no piense que los datos son viejos y vuelva a lanzar el calculo rapidamente
	structBetType.LastUpdate = time.Now().UnixMilli()
	// Limpiar cuotas después de calcular surebets
	//structBetType.data = make(map[string]map[string]map[string]float64)
	//fmt.Printf("🔄 Cuotas de %s en partido %s eliminadas tras cálculo.\n", betType, matchID)
	structBetType.mu.Unlock()
}
