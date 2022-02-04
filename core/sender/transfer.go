package sender

import (
	"io"
	"fmt"
	"log"
	"bufio"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/abdfnx/tran/tools"
	"github.com/abdfnx/tran/constants"
	"github.com/abdfnx/tran/models/protocol"
)

// Transfer is the file transfer sequence, can be via relay or tranx.
func (s *Sender) Transfer(wsConn *websocket.Conn) error {
	s.state = WaitForFileRequest

	for {
		// Read incoming message.
		receivedMsg, err := tools.ReadEncryptedMessage(wsConn, s.crypt)
		if err != nil {
			wsConn.Close()
			s.closeServer <- syscall.SIGTERM
			return fmt.Errorf("shutting down tran due to websocket error: %s", err)
		}

		// main switch for action based on incoming message.
		// The states flows from top down. States checks are performend at each step.
		switch receivedMsg.Type {
			case protocol.ReceiverRequestPayload:
				if s.state != WaitForFileRequest {
					err = tools.WriteEncryptedMessage(wsConn, protocol.TransferMessage{
						Type:    protocol.TransferError,
						Payload: fmt.Sprintf("Tran unsynchronized, expected state: %s, actual: %s", WaitForFileRequest.Name(), s.state.Name()),
					}, s.crypt)

					if err != nil {
						return err
					}

					wsConn.Close()
					s.closeServer <- syscall.SIGTERM

					return NewWrongStateError(WaitForFileRequest, s.state)
				}

				err = s.streamPayload(wsConn)
				if err != nil {
					log.Println("error in payload streaming:", err)

					return err
				}

				err = tools.WriteEncryptedMessage(wsConn, protocol.TransferMessage{
					Type:    protocol.SenderPayloadSent,
					Payload: "Tran transfer completed",
				}, s.crypt)

				if err != nil {
					return err
				}

				s.state = WaitForFileAck
				s.updateUI()

			case protocol.ReceiverPayloadAck:
				if s.state != WaitForFileAck {
					err = tools.WriteEncryptedMessage(wsConn, protocol.TransferMessage{
						Type:    protocol.TransferError,
						Payload: fmt.Sprintf("Tran unsynchronized, expected state: %s, actual: %s", WaitForFileAck.Name(), s.state.Name()),
					}, s.crypt)

					if err != nil {
						return err
					}

					wsConn.Close()
					s.closeServer <- syscall.SIGTERM

					return NewWrongStateError(WaitForFileAck, s.state)
				}

				s.state = WaitForCloseMessage
				s.updateUI()

				err = tools.WriteEncryptedMessage(wsConn, protocol.TransferMessage{
					Type:    protocol.SenderClosing,
					Payload: "Closing down Tran as requested",
				}, s.crypt)

				if err != nil {
					return err
				}

				s.state = WaitForCloseAck
				s.updateUI()

			case protocol.ReceiverClosingAck:
				wsConn.Close()
				s.closeServer <- syscall.SIGTERM

				if s.state != WaitForCloseAck {
					return NewWrongStateError(WaitForCloseAck, s.state)
				}

				return nil

			case protocol.TransferError:
				s.updateUI()
				log.Println("Shutting down Tran due to a transfer error")
				wsConn.Close()
				s.closeServer <- syscall.SIGTERM

				return fmt.Errorf("TransferError during file transfer")
		}
	}
}

// streamPayload streams the payload over the provided websocket connection while reporting the progress.
func (s *Sender) streamPayload(wsConn *websocket.Conn) error {
	bufReader := bufio.NewReader(s.payload)
	chunkSize := ChunkSize(s.payloadSize)
	buffer := make([]byte, chunkSize)

	var bytesSent int

	for {
		n, err := bufReader.Read(buffer)
		bytesSent += n
		enc, encErr := s.crypt.Encrypt(buffer[:n])

		if encErr != nil {
			return encErr
		}

		wsConn.WriteMessage(websocket.BinaryMessage, enc)
		progress := float32(bytesSent) / float32(s.payloadSize)
		s.updateUI(progress)

		if err == io.EOF {
			break
		}
	}

	return nil
}

// ChunkSize returns an appropriate chunk size for the payload size
func ChunkSize(payloadSize int64) int64 {
	// clamp amount of chunks to be at most MAX_SEND_CHUNKS if it exceeds
	if payloadSize / constants.MAX_CHUNK_BYTES > constants.MAX_SEND_CHUNKS {
		return int64(payloadSize) / constants.MAX_SEND_CHUNKS
	}
	// if not exceeding MAX_SEND_CHUNKS, divide up no. of chunks to MAX_CHUNK_BYTES-sized chunks
	chunkSize := int64(payloadSize) / constants.MAX_CHUNK_BYTES
	// clamp amount of chunks to be at least MAX_CHUNK_BYTES
	if chunkSize <= constants.MAX_CHUNK_BYTES {
		return constants.MAX_CHUNK_BYTES
	}

	return chunkSize
}
