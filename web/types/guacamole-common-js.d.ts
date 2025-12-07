declare module 'guacamole-common-js' {
    export class Client {
        constructor(tunnel: Tunnel);
        connect(params?: string): void;
        disconnect(): void;
        sendSize(width: number, height: number): void;
        getDisplay(): Display;
        onstatechange: (state: number) => void;
        onerror: (status: Status) => void;
        onclipboard: (stream: InputStream, mimetype: string) => void;
    }

    export class Tunnel {
        onstatechange: (state: number) => void;
        onerror: (status: Status) => void;
        oninstruction: (opcode: string, args: any[]) => void;
        connect(data: string): void;
        disconnect(): void;
        sendMessage(opcode: string, ...args: any[]): void;
        state: number;
        setState(state: number): void;
        static State: {
            CONNECTING: number;
            OPEN: number;
            CLOSED: number;
            UNSTABLE: number;
        };
    }

    export class WebSocketTunnel extends Tunnel {
        constructor(url: string);
    }

    export class HTTPTunnel extends Tunnel {
        constructor(url: string);
    }

    export class Display {
        getElement(): HTMLElement;
        scale(scale: number): void;
        getWidth(): number;
        getHeight(): number;
    }

    export class Mouse {
        constructor(element: HTMLElement);
        onmousedown: (state: MouseState) => void;
        onmouseup: (state: MouseState) => void;
        onmousemove: (state: MouseState) => void;
    }

    export class Keyboard {
        constructor(element: HTMLElement | Document);
        onkeydown: (keysym: number) => void;
        onkeyup: (keysym: number) => void;
    }

    export class InputStream {
        onblob: (blob: string) => void;
        onend: () => void;
    }

    export class Status {
        code: number;
        message: string;
        originalEvent?: any;
    }

    export class MouseState {
        x: number;
        y: number;
        left: boolean;
        middle: boolean;
        right: boolean;
        up: boolean;
        down: boolean;
    }

    export class Parser {
        constructor();
        receive(data: string): void;
        oninstruction: (opcode: string, args: any[]) => void;
    }
}
