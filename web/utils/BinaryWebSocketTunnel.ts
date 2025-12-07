import Guacamole from 'guacamole-common-js';

export class BinaryWebSocketTunnel extends Guacamole.Tunnel {
    private socket: WebSocket | null = null;
    private decoder: TextDecoder;
    private tunnelUrl: string;
    private parser: Guacamole.Parser;

    constructor(tunnelUrl: string) {
        super();
        console.log("BinaryWebSocketTunnel: Constructor called with", tunnelUrl);
        this.tunnelUrl = tunnelUrl;
        this.decoder = new TextDecoder("utf-8");
        this.parser = new Guacamole.Parser();

        this.parser.oninstruction = (opcode, args) => {
            // console.log("Tunnel instruction:", opcode, args);
            if (this.oninstruction) {
                this.oninstruction(opcode, args);
            }
        };
    }

    connect = (data: string): void => {
        console.log("BinaryWebSocketTunnel: Connecting...");
        this.socket = new WebSocket(this.tunnelUrl, "guacamole");
        this.socket.binaryType = "arraybuffer";

        this.socket.onopen = () => {
            // console.log("BinaryWebSocketTunnel: WebSocket Open");
            // Reset decoder on new connection
            this.decoder = new TextDecoder("utf-8");
            this.setState(Guacamole.Tunnel.State.OPEN);
        };

        this.socket.onclose = (event) => {
            console.log("BinaryWebSocketTunnel: WebSocket Closed", event.code, event.reason);
            this.setState(Guacamole.Tunnel.State.CLOSED);
        };

        this.socket.onerror = (event) => {
            console.error("BinaryWebSocketTunnel: WebSocket Error", event);
            if (this.onerror) {
                this.onerror({
                    code: 500, // generic error
                    message: "WebSocket error",
                    originalEvent: event
                });
            }
        };

        this.socket.onmessage = (event) => {
            // console.log("BinaryWebSocketTunnel: Message received", event.data);
            if (event.data instanceof ArrayBuffer) {
                const text = this.decoder.decode(event.data, { stream: true });
                this.parser.receive(text);
            } else if (typeof event.data === "string") {
                this.parser.receive(event.data);
            }
        };
    }

    sendMessage = (message: string) => {
        // console.log("Sending message:", message);
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(message);
        } else {
            console.warn("Socket not open, cannot send:", message);
        }
    };

    sendInstruction = (opcode: string, args: any[]) => {
        let message = `${opcode.length}.${opcode}`;
        for (const arg of args) {
            const argStr = String(arg);
            message += `,${argStr.length}.${argStr}`;
        }
        message += ';';
        this.sendMessage(message);
    };

    disconnect(): void {
        console.log('BinaryWebSocketTunnel: disconnect() called, socket state:', this.socket?.readyState)
        if (this.socket) {
            console.log('BinaryWebSocketTunnel: Closing websocket...')
            this.socket.close();
            this.socket = null;
        } else {
            console.log('BinaryWebSocketTunnel: No socket to close')
        }
    }
}
