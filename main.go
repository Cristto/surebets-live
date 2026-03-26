package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	domain "github.com/Cristto/surebets-live/internal"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

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

	dataBet := domain.NewDataBet()
	channelSurebet := make(chan domain.Surebet)
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
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for data := range buffer {
				//fmt.Printf("[👷 Worker %d] Procesando entrada: %s\n", workerID, data)
				dataBet.SaveEntry(data, channelSurebet)
			}
		}(i) // Pasamos el índice del worker (para depuración si es necesario)
	}

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
