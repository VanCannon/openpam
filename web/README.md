# OpenPAM Web Frontend

Next.js web application for the OpenPAM Privileged Access Management system.

## Features

- **Authentication**: Microsoft EntraID/Azure AD OAuth2 integration
- **SSH Access**: Browser-based SSH terminal using xterm.js
- **RDP Access**: Browser-based RDP viewer using Guacamole protocol
- **Target Management**: Browse and connect to available targets
- **Admin Dashboard**: Manage zones, targets, credentials, and view audit logs
- **Real-time Sessions**: WebSocket-based connections to backend gateway

## Prerequisites

- Node.js 18+ and npm
- OpenPAM Gateway backend running (see `/gateway`)

## Installation

```bash
# Install dependencies
npm install
```

## Configuration

Create a `.env.local` file based on `.env.local.example`:

```bash
# API Configuration
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080

# EntraID Configuration
NEXT_PUBLIC_ENTRA_CLIENT_ID=your-client-id-here
NEXT_PUBLIC_ENTRA_TENANT_ID=your-tenant-id-here
NEXT_PUBLIC_ENTRA_REDIRECT_URL=http://localhost:3000/auth/callback
```

## Development

```bash
# Run development server
npm run dev
```

The application will be available at http://localhost:3000

## Production Build

```bash
# Build for production
npm run build

# Start production server
npm start
```

## Project Structure

```
web/
├── app/                    # Next.js app directory
│   ├── auth/              # Authentication pages
│   ├── dashboard/         # Main dashboard
│   ├── admin/             # Admin pages
│   │   ├── zones/        # Zone management
│   │   ├── targets/      # Target management
│   │   ├── credentials/  # Credential management (TODO)
│   │   └── audit/        # Audit log viewer
│   ├── layout.tsx        # Root layout
│   └── page.tsx          # Home page
├── components/            # React components
│   ├── terminal.tsx      # SSH terminal component
│   └── rdp-viewer.tsx    # RDP viewer component
├── lib/                   # Libraries and utilities
│   ├── api.ts            # API client
│   └── auth-context.tsx  # Authentication context
└── types/                 # TypeScript type definitions
    └── index.ts
```

## User Flow

1. **Login**: User clicks "Sign in with Microsoft" → redirected to EntraID
2. **Callback**: EntraID redirects back with token → stored and user authenticated
3. **Dashboard**: User sees list of available targets
4. **Connect**: User selects target → chooses credential → opens SSH/RDP session
5. **Session**: WebSocket connection established to gateway → data proxied to target

## Admin Features

Administrators can manage:

- **Zones**: Create hub and satellite zones
- **Targets**: Add SSH and RDP targets to zones
- **Credentials**: Link Vault secret paths to targets (TODO)
- **Audit Logs**: View all connection history with bytes transferred

## Components

### Terminal Component

- Built with xterm.js
- Full terminal emulation (colors, cursor movement, etc.)
- Automatic resizing
- Connection status indicator
- WebSocket-based communication

### RDP Viewer Component

- Guacamole protocol integration
- Mouse and keyboard event forwarding
- Connection status indicator
- WebSocket-based communication

## API Integration

The frontend communicates with the gateway backend via:

- **REST API**: `/api/v1/*` endpoints for CRUD operations
- **WebSocket**: `/api/ws/connect/{protocol}/{target_id}` for sessions

All API calls include JWT token in Authorization header or cookie.

## Styling

- Tailwind CSS for utility-first styling
- Responsive design for mobile and desktop
- Dark mode support for terminal

## Security

- JWT tokens stored in localStorage and httpOnly cookies
- All API requests require authentication
- WebSocket connections authenticated with JWT
- Credentials never exposed to frontend (retrieved by backend from Vault)

## Browser Support

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

## Troubleshooting

### Authentication Fails

- Verify EntraID configuration matches backend
- Check redirect URL is correctly configured in Azure AD
- Ensure backend is running and accessible

### WebSocket Connection Fails

- Check `NEXT_PUBLIC_WS_URL` matches gateway address
- Verify target exists and is enabled
- Ensure credential is valid

### Terminal Not Displaying

- Check browser console for errors
- Verify xterm.js loaded correctly
- Ensure WebSocket connection established
