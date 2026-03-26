package domain

import (
	"fmt"
	"time"
)

// comprueba si el tiempo entre que metio la ultima cuota y el actual superan los 2 segundos
// si bm.Updated que representa los tipos de apuestas tienen cuotas metidas
// y si no esta calculando surebets con calculating
func (bm *BetMatch) MonitorInactivity(matchID string, channelSurebet chan Surebet) {
	for {
		// se activa cada 500 ms
		time.Sleep(500 * time.Millisecond)

		// recorremos como un diccionario todas las estructuras de BetMatch
		// para saber cual ha dejado de ingresar cuotas
		for betType, dataBetType := range map[string]*DataBetType{
			"BTG": bm.BTG,
			"TPI": bm.TPI,
			"TUO": bm.TUO,
			"FUO": bm.FUO,
		} {
			dataBetType.mu.Lock()
			// si compruebas dataBetType sin mas aunque parezca vacio siempre tendra valor por sus campos que alguno esta siempre inicializado
			hasBets := len(dataBetType.data) > 0
			// elapsed calcula la ultima vez que introdujo datos en la estructura
			elapsed := time.Now().UnixMilli() - dataBetType.LastUpdate
			shouldCalculate := hasBets && elapsed > 2000 && !dataBetType.Calculating
			dataBetType.mu.Unlock()
			if shouldCalculate {
				fmt.Printf("Calculando surebets para %s en partido %s\n", betType, matchID)
				go bm.CalculateSurebets(dataBetType, betType, matchID, channelSurebet) // Pasamos la estructura específica y usamos gorrutina al calcular
			}
		}
	}
}

func AddBet(structBetType *DataBetType, house, ident, header string, odd float64) {

	// esto es para evitar problemas de concurrencia al modificar la estructura
	structBetType.mu.Lock()
	defer structBetType.mu.Unlock()

	if _, ok := structBetType.data[house]; !ok {
		structBetType.data[house] = make(map[string]map[string]float64) // esto es un mapa de mapas
	}

	if _, ok := structBetType.data[house][ident]; !ok {
		structBetType.data[house][ident] = make(map[string]float64) // es un mapa de cuotas
	}

	if _, ok := structBetType.data[house][ident][header]; !ok {
		structBetType.data[house][ident][header] = odd
		return
	}

	// Si la cuota es diferente se actualiza
	if structBetType.data[house][ident][header] != odd {
		structBetType.data[house][ident][header] = odd
		return
	}

	// si la cuota ya estaba no hace nada

}
