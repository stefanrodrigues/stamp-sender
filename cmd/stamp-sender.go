package main

import (
        "flag"
        "fmt"
        "log"
        "math"
        "net"
        "os"
        "time"
        "github.com/stefanrodrigues/stamp"
        "encoding/binary"
        "encoding/hex"
)

func main() {
        // Definição dos flags da linha de comando
        target := flag.String("target", "", "O endereço IP ou hostname do refletor TWAMP (obrigatório)")
        port := flag.Int("port", stamp.DefaultPort, "A porta UDP do refletor")
        count := flag.Int("count", 5, "O número de pacotes de teste a serem enviados")
        interval := flag.Duration("interval", 1*time.Second, "O intervalo entre o envio de pacotes (ex: 200ms)")
        timeout := flag.Duration("timeout", 2*time.Second, "O tempo de espera por respostas após o envio do último pacote")
        flag.Parse()

        if *target == "" {
                fmt.Println("Erro: O argumento --target é obrigatório.")
                flag.Usage()
                os.Exit(1)
        }

        // Resolução do endereço do refletor
        raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", *target, *port))
        if err != nil {
                log.Fatalf("Erro ao resolver o endereço do refletor: %v", err)
        }

        // Conexão UDP
        conn, err := net.DialUDP("udp", nil, raddr)
        if err != nil {
                log.Fatalf("Erro ao conectar ao refletor: %v", err)
        }
        defer conn.Close()

        fmt.Printf("Enviando %d pacotes TWAMP para %s a cada %v...\n\n", *count, raddr.String(), *interval)

        // Map para armazenar o tempo de envio de cada pacote
        sentPackets := make(map[uint32]time.Time)
        // Slice para armazenar os resultados de RTT
        results := []time.Duration{}

        // Fase de Envio
        for i := 0; i < *count; i++ {
                seq := uint32(i)
                packet := &stamp.StampPacket{
                        Sequence:  seq,
                        SenderTTL: 64, // TTL típico
                }

                // Armazenar o tempo de envio (T1) o mais próximo possível do envio real
                t1 := time.Now()
                packet.Timestamp = stamp.ToNTP(t1)
                sentPackets[seq] = t1

                // Serializar e enviar o pacote
                bytes, err := packet.ToBytes()
                if err != nil {
                        log.Printf("Erro ao serializar pacote %d: %v", seq, err)
                        continue
                }
                //Envia Pacotes --> Write
                if _, err := conn.Write(bytes); err != nil {
                        log.Printf("Erro ao enviar pacote %d: %v", seq, err)
                }

                time.Sleep(*interval)
        }

        // Fase de Recebimento
        // Continuaremos a escutar por um tempo definido pelo timeout após o último envio.
        conn.SetReadDeadline(time.Now().Add(*timeout))
        for len(sentPackets) > 0 {
                respBuf := make([]byte, 1500)
                n, _, err := conn.ReadFromUDP(respBuf)
                if err != nil {
                        if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                                // O timeout foi atingido, encerramos a fase de recebimento
                                break
                        }
                        log.Printf("Erro ao ler resposta: %v", err)
                        continue
                }

                // Obter o tempo de chegada (T4)
                t4 := time.Now()

                //Carrega packet no responsePacket (FromBytes)
                responsePacket := new(stamp.StampPacket)
                if err := responsePacket.FromBytes(respBuf[:n]); err != nil {
                        log.Printf("Erro ao desserializar pacote de resposta: %v", err)
                        //DEBUG
                        log.Printf("Tamanho da struct: %d, Tamanho do buffer: %d\n", binary.Size(responsePacket), len(respBuf[:n]))
                        dumpFormatado := hex.Dump(respBuf[:n])
                        log.Println(dumpFormatado)
                        //END_DEBUG
                        continue
                }

                // Verificar se o pacote recebido corresponde a um que enviamos
                if _, ok := sentPackets[responsePacket.SenderSequence]; ok {
                        //Calcula RTT (RTT --> packet.go)
                        rtt := responsePacket.RTT(t4)
                        fmt.Printf("Resposta de %s: seq=%d, rtt=%v\n", *target, responsePacket.SenderSequence, rtt)
                        fmt.Printf("T1:%v\nT2:%v\nT3:%v\nT4:%v\n",stamp.FromNTP(responsePacket.SenderTimestamp),stamp.FromNTP(responsePacket.ReceiveTimestamp),stamp.FromNTP(responsePacket.Timestamp),t4)
                        results = append(results, rtt)
                        // Remover o pacote do map para marcá-lo como recebido
                        delete(sentPackets, responsePacket.SenderSequence)
                } else {
                        log.Printf("Recebido pacote inesperado com seq=%d", responsePacket.SenderSequence)
                }
        }

        // Fase de Análise e Impressão dos Resultados
        fmt.Println("\n--- Estatísticas do Teste ---")
        packetsSent := *count
        packetsRecv := len(results)
        packetsLost := packetsSent - packetsRecv
        lossPercent := float64(packetsLost) / float64(packetsSent) * 100

        fmt.Printf("Pacotes: Enviados = %d, Recebidos = %d, Perdidos = %d (%.2f%% de perda)\n",
                packetsSent, packetsRecv, packetsLost, lossPercent)

        if packetsRecv > 0 {
                var min, max, avg, total time.Duration
                min = results[0]
                max = results[0]

                for _, rtt := range results {
                        if rtt < min {
                                min = rtt
                        }
                        if rtt > max {
                                max = rtt
                        }
                        total += rtt
                }
                avg = total / time.Duration(packetsRecv)

                // Cálculo do Jitter (desvio padrão dos RTTs)
                var sumOfSquares time.Duration
                for _, rtt := range results {
                        diff := rtt - avg
                        sumOfSquares += diff * diff
                }
                jitter := time.Duration(math.Sqrt(float64(sumOfSquares / time.Duration(packetsRecv))))

                fmt.Printf("RTT (ida e volta): min/avg/max/jitter = %v/%v/%v/%v\n", min, avg, max, jitter)
        }
}
