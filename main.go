package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var rebootPending = false

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("No se pudo cargar .env")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("Falta la variable TELEGRAM_BOT_TOKEN")
	}

	allowedUserID, err := strconv.Atoi(os.Getenv("TELEGRAM_USER_ID"))
	if err != nil {
		log.Fatal("error leyendo TELEGRAM_USER_ID")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Bot conectado como %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if rebootPending {
			if !update.Message.IsCommand() || update.Message.Command() != "confirm" {
				rebootPending = false
			}
		}

		if update.Message.From.ID != int64(allowedUserID) {
			log.Printf("Ignorando mensaje de usuario no permitido. Usuario: %s, ID: %d", update.Message.From.UserName, update.Message.From.ID)
			continue
		}
		var respuesta string

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				respuesta = "Hola 👋 Soy tu bot en la Raspberry Pi 🤖"
			case "ping":
				respuesta = "pong 🏓"
			case "uptime":
				respuesta, err = uptime()
				if err != nil {
					respuesta = "❌ No se pudo obtener el uptime"
				}
			case "temp":
				respuesta, err = cpuTemp()
				if err != nil {
					respuesta = "❌ No se pudo obtener la temperatura de CPU"
				}
			case "disk":
				respuesta, err = diskUsage()
				if err != nil {
					respuesta = "❌ No se pudo obtener el uso de disco"
				}
			case "ram":
				respuesta, err = ramUsage()
				if err != nil {
					respuesta = "❌ No se pudo obtener el uso de RAM"
				}
			case "reboot":
				rebootPending = true
				respuesta = "⚠️ ¿Seguro? Escribe /confirm para reiniciar"
			case "confirm":
				if rebootPending {
					respuesta = "♻️ Reiniciando..."
					go reboot()
					rebootPending = false
				} else {
					respuesta = "No hay ninguna acción pendiente"
				}
			case "help":
				respuesta = "/start\n" +
					"/ping\n" +
					"/uptime\n" +
					"/temp\n" +
					"/disk\n" +
					"/ram\n" +
					"/reboot\n" +
					"/help"
			default:
				respuesta = "No conozco ese comando, prueba con /help"
			}
		} else {
			respuesta = "Muy interesante... pero no soy de muchas palabras prueba con /help "
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, respuesta)
		bot.Send(msg)
	}
}

func uptime() (string, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}

	fields := strings.Fields(string(data))
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", err
	}

	d := time.Duration(seconds) * time.Second

	dias := d / (24 * time.Hour)
	d -= dias * 24 * time.Hour
	horas := d / time.Hour
	d -= horas * time.Hour
	minutos := d / time.Minute

	return fmt.Sprintf("⏱️ Uptime: %dd %dh %dm", dias, horas, minutos), nil
}

func cpuTemp() (string, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return "", err
	}

	milli, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return "", err
	}

	temp := float64(milli) / 1000.0
	return fmt.Sprintf("🌡️ Temperatura CPU: %.1f°C", temp), nil
}

func diskUsage() (string, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return "", err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free

	gb := func(b uint64) uint64 { return b / 1024 / 1024 / 1024 }

	return fmt.Sprintf(
		"💾 Disco:\nUsado: %d GB\nLibre: %d GB\nTotal: %d GB",
		gb(used), gb(free), gb(total),
	), nil
}

func ramUsage() (string, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	mem := make(map[string]int)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.Atoi(fields[1])
		mem[strings.TrimSuffix(fields[0], ":")] = val
	}

	total := mem["MemTotal"] / 1024    // MB
	free := mem["MemAvailable"] / 1024 // MB
	used := total - free

	return fmt.Sprintf("🧠 RAM:\nUsada: %d MB\nLibre: %d MB\nTotal: %d MB", used, free, total), nil
}

func reboot() error {
	cmd := exec.Command("sudo", "reboot")
	return cmd.Run()
}
