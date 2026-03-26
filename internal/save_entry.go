package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func RemoveAccents(s string) string {
	t := norm.NFD.String(s) // Descompone caracteres con acentos
	return strings.Map(func(r rune) rune {
		if unicode.IsMark(r) { // Filtra los acentos
			return -1
		}
		return r
	}, t)
}

// posible entrada: Bet365:Real Madrid:Barcelona:btg:Sí:xxx:1.95
// solo los de mas menos tienen header tambien
// bet:Girona:Las Palmas:tuo:2,5:Mas:4.75
func (a *DataBet) SaveEntry(entrie string, channelSurebet chan Surebet) {

	// primero divide el string en partes
	parts := strings.Split(entrie, ":")
	if len(parts) != 7 {
		//return Apuesta{}, fmt.Errorf("entrada inválida: %s", entrada)
		fmt.Printf("[ERROR] invalid entrie: %s", entrie)
		return
	}

	house, homeTeam, awayTeam, betType, ident, header, oddStr := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]
	// la cuota se pasa de string a float siempre
	odd, err := strconv.ParseFloat(strings.Replace(oddStr, ",", ".", 1), 64)
	if err != nil {
		fmt.Printf("[ERROR] could not convert the quota: %s||%s\n", oddStr, entrie)
		return
	}

	homeTeam = RemoveAccents(homeTeam)
	awayTeam = RemoveAccents(awayTeam)

	keyTeam := homeTeam + ":" + awayTeam

	// esta busqueda la hace siempre
	a.mu.Lock()
	match, exists := a.Matches[keyTeam] // Buscamos el partido si no existe se crea
	if !exists {
		match = NewBetMatch()
		a.Matches[keyTeam] = match                          // se asocia como valor las estructuras del partido a la clave del partido
		go match.MonitorInactivity(keyTeam, channelSurebet) // se crea una gorrutina que solo monitoriza este partido
	}
	a.mu.Unlock()

	// se elige la estructura segun el tipo de cuota
	var structBetType *DataBetType
	switch betType {
	case "btg":
		structBetType = match.BTG
	case "tpi":
		structBetType = match.TPI
	case "tuo":
		structBetType = match.TUO
	case "fuo":
		structBetType = match.FUO
	default:
		fmt.Println("Invalid bet type")
		return
	}

	// bloquea la estructura especifica y se comprueba si esta calculando surebets
	structBetType.mu.Lock()
	if structBetType.Calculating {
		structBetType.mu.Unlock()
		fmt.Printf("No se pueden agregar apuestas a %s, calculando surebets...\n", betType)
		return
	}
	structBetType.mu.Unlock()

	AddBet(structBetType, house, ident, header, odd)
	fmt.Printf("Añadida cuota nueva %s\n", entrie)

	// Marca la última actualización y la apuesta como actualizada
	// para que se vuelva a calcular el tiempo antes de sacar sures.
	structBetType.mu.Lock()
	structBetType.LastUpdate = time.Now().UnixMilli()
	structBetType.mu.Unlock()

	/*
		ya no hay Updated[betype] = true donde añadias que tipo de apuesta habia recibido cuota
		ahora se mira cada una de ellas para saberlo
	*/
}
