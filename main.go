package main

import (
	"context"
	"fmt"

	//"hash/fnv"
	"log"
	"os"
	"unicode"

	"os/signal"
	"strconv"
	"strings"
	"sync"

	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"golang.org/x/text/unicode/norm"
)

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

func RemoveAccents(s string) string {
	t := norm.NFD.String(s) // Descompone caracteres con acentos
	return strings.Map(func(r rune) rune {
		if unicode.IsMark(r) { // Filtra los acentos
			return -1
		}
		return r
	}, t)
}

/*
ahora solo bloqueamos el tipo de estructura para que calcule las sures
ademas la funcion es llamada con una gorrutina para que no interfiera en las demas
Cada estrucura de DataBetType tiene su calculating para que solo le afecte a ella
*/
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

var ctx = context.Background()

func main() {

	// Cargar variables guardadas en el archivo .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error cargando el archivo .env")
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		log.Fatal("Faltan variables de entorno, verifica TELEGRAM_BOT_TOKEN y TELEGRAM_CHAT_ID")
	}

	// conectarnos con redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASS"),
		DB:       0, // Use default DB
	})

	// Probar la conexión
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	fmt.Println("✅ Conexión exitosa a Redis:", pong)

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	// ID del usuario o chat al que enviar las surebets
	ID, _ := strconv.ParseInt(chatID, 10, 64)

	// Mensaje con una surebet
	//message := "💰 Nueva Surebet encontrada!\nEvento: Equipo A vs Equipo B\nCuotas: 2.1 | 1.95\nGanancia asegurada: 5%"

	// Enviar mensaje
	//msg := tgbotapi.NewMessage(ID, message)
	//_, err = bot.Send(msg)
	//if err != nil {
	//	log.Println("Error enviando mensaje:", err)
	//} else {
	//	log.Println("Mensaje enviado correctamente")
	//}

	dataBet := NewDataBet()
	channelSurebet := make(chan Surebet)
	var wg sync.WaitGroup

	// Canal de buffer para las entradas de Redis
	buffer := make(chan string, 100)

	// Goroutine para consumir Redis y llenar el buffer
	go func() {
		subscriber := rdb.Subscribe(ctx, "surebets")
		defer subscriber.Close()
		fmt.Println("📡 Escuchando mensajes de Redis...")

		for {
			msg, err := subscriber.ReceiveMessage(ctx)
			if err != nil {
				log.Println("❌ Error recibiendo mensaje de Redis:", err)
				continue
			}

			// Enviar mensaje al buffer
			buffer <- msg.Payload
		}
	}()

	// Pool de goroutines para procesar mensajes

	numWorkers := 5 // Número de goroutines procesadoras

	for i := 0; i < numWorkers; i++ { // Lanzamos múltiples goroutines
		wg.Add(1)   // Incrementamos el contador de WaitGroup para esperar su finalización
		go func() { // Cada goroutine es un worker
			defer wg.Done()            // Cuando termine, avisa al WaitGroup para continuar
			for data := range buffer { // Escucha y procesa datos del buffer continuamente
				dataBet.SaveEntry(data, channelSurebet) // Procesa cada entrada
			}
		}()
	}

	// si lo quiero para depurar uso esta, asi viendo que worker trabaja cada gorrutina
	/*for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for data := range buffer {
				//fmt.Printf("[👷 Worker %d] Procesando entrada: %s\n", workerID, data)
				dataBet.SaveEntry(data, channelSurebet)
			}
		}(i) // Pasamos el índice del worker (para depuración si es necesario)
	}*/

	// Goroutine para leer el canal de surebets y evitar deadlock
	go func() {
		cont := 0
		for surebet := range channelSurebet {
			cont++
			message := fmt.Sprintf(
				"[%d] 💰 Surebet encontrada:\n  [%s, Ident: %s, Header: %s, Cuota: %.2f]\n  [%s, Ident: %s, Header: %s, Cuota: %.2f]\n  Valor: %.4f, Ganancia: %.2f%%\n\n",
				cont,
				surebet.House, surebet.Ident, surebet.Header, surebet.Odd,
				surebet.OppositeHouse, surebet.OppositeIdent, surebet.OppositeHeader, surebet.OppositeOdd,
				surebet.Surebet, surebet.Percent,
			)
			msg := tgbotapi.NewMessage(ID, message)
			if _, err = bot.Send(msg); err != nil {
				fmt.Println("Error al mandar mensaje a telegram")
			}
		}
	}()

	// Capturar señales del sistema para cerrar el programa de forma segura
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	<-signalChan // Espera hasta que se reciba una señal de interrupción

	fmt.Println("\n🛑 Señal recibida. Cerrando programa...")

	// Cerrar canales y esperar a que terminen las goroutines
	//close(buffer)
	wg.Wait()
	close(channelSurebet)

	fmt.Println("✅ Programa cerrado correctamente.")
}
