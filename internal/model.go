package domain

import "sync"

// una apuesta sola
type Bet struct {
	House  string
	Ident  string
	Header string
	Odd    float64
}

// la estructura resultante cuando existe una surebet
type Surebet struct {
	KeyTeam        string
	House          string
	Ident          string
	Header         string
	Odd            float64
	OppositeHouse  string
	OppositeIdent  string
	OppositeHeader string
	OppositeOdd    float64
	Surebet        float64
	Percent        float64
}

// key: homeTeam:awayTeam
// aqui empieza Partidos -> map[string] -> *ApuestasPorPartido
type DataBet struct {
	Matches map[string]*BetMatch
	mu      sync.Mutex // protege la estructura cuando entra un partido nuevo
}

// los tipos de apuestas sobre un evento deportivo concreto
// BTG -> DataBetType
type BetMatch struct {
	BTG *DataBetType
	TPI *DataBetType
	TUO *DataBetType
	FUO *DataBetType
	mu  sync.Mutex // Mutex para evitar acceso concurrente al partido
}

// bwi:Girona:Las Palmas:btg:No:xxx:1.19
// bet:Girona:Las Palmas:tuo:2,5:Mas:4.75
// bet:Girona:Las Palmas:tuo:1,5:Menos:2.3
// data["Bet365"]["TUO"]["Sí"]["xxx"] = 1.85
// con mutex indico que solo las gorrutinas puedan acceder a la estructura de una en una
// en cada funcion que manipule la estructura agregamos las funciones Lock y Unlock
type DataBetType struct {
	data        map[string]map[string]map[string]float64 // Casa -> Ident -> header -> Cuota
	mu          sync.Mutex
	Calculating bool  // NUEVO: Cada estructura maneja su propio cálculo de surebets
	LastUpdate  int64 // Última actualización en milisegundos
}

// Constructores, sin esto el mapa estaria a nil y no podrias escribir en ellos
func NewDataBetType() *DataBetType {
	return &DataBetType{
		data: make(map[string]map[string]map[string]float64),
	}
}

func NewBetMatch() *BetMatch {
	return &BetMatch{
		BTG: NewDataBetType(),
		TPI: NewDataBetType(),
		TUO: NewDataBetType(),
		FUO: NewDataBetType(),
	}
}

func NewDataBet() *DataBet {
	return &DataBet{
		Matches: make(map[string]*BetMatch),
	}
}
