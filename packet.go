package stamp

import (
        "bytes"
        "encoding/binary"
        "time"
)

const (
        // DefaultPort é a porta UDP padrão para TWAMP e TWAMP Light.
        DefaultPort = 6000
)


// RFC 8762
// 4.3.1. Session-Reflector Packet Format in Unauthenticated Mode
type StampPacket struct {
        Sequence         uint32  //Packet sequence number
        Timestamp        uint64  // Formato NTP
        ErrorEstimate    uint16
        MBZ              [2]byte // Must be zero
        ReceiveTimestamp uint64  // Formato NTP
        SenderSequence   uint32  // Value copied from Sequence field
        SenderTimestamp  uint64  // Formato NTP
        SenderErrorEst   uint16  // Value copied from ErrorEstimate field
        MBZ2             [2]byte // Must be zero
        SenderTTL        byte
        MBZ3             [15]byte // Must be zero
}

// ToBytes serializa o pacote para um slice de bytes para envio.
func (p *StampPacket) ToBytes() ([]byte, error) {
        buf := new(bytes.Buffer)
        if err := binary.Write(buf, binary.BigEndian, p); err != nil {
                return nil, err
        }
        return buf.Bytes(), nil
}

// FromBytes desserializa um slice de bytes para a estrutura do pacote.
func (p *StampPacket) FromBytes(data []byte) error {
        buf := bytes.NewReader(data)
        if err := binary.Read(buf, binary.BigEndian, p); err != nil {
                return err
        }
        return nil
}

// RTT calcula o Round-Trip Time (tempo de ida e volta) com base nos timestamps do pacote.
// A fórmula é: RTT = (T4 - T1) - (T3 - T2)
// Onde T4 é o tempo em que o sender recebe a resposta.
func (p *StampPacket) RTT(t4 time.Time) time.Duration {
        t1 := FromNTP(p.SenderTimestamp)
        t2 := FromNTP(p.ReceiveTimestamp)
        t3 := FromNTP(p.Timestamp) // No pacote de resposta, este campo é T3.

        // Calcula o tempo total de ida e volta
        totalDuration := t4.Sub(t1)
        // Calcula o tempo de processamento no refletor
        reflectorProcessingDuration := t3.Sub(t2)

        // O RTT real é o tempo total menos o tempo de processamento do refletor.
        // Se os relógios não estiverem sincronizados, reflectorProcessingDuration pode ser negativo.
        // A fórmula se mantém correta mesmo com clocks não sincronizados.
        return totalDuration - reflectorProcessingDuration
}
