/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080',
  },
  turbopack: {
    root: '/home/billy/openpam/web',
  },
  async rewrites() {
    return [
      {
        source: '/api/v1/identity/:path*',
        destination: 'http://localhost:8082/api/v1/identity/:path*',
      },
      {
        source: '/api/v1/users',
        destination: 'http://localhost:8082/api/v1/users',
      },
      {
        source: '/api/v1/computers',
        destination: 'http://localhost:8082/api/v1/computers',
      },
      {
        source: '/api/v1/ad-users',
        destination: 'http://localhost:8082/api/v1/ad-users',
      },
      {
        source: '/api/v1/ad-computers',
        destination: 'http://localhost:8082/api/v1/ad-computers',
      },
      {
        source: '/api/v1/users/import',
        destination: 'http://localhost:8082/api/v1/users/import',
      },
      {
        source: '/api/v1/managed-accounts',
        destination: 'http://localhost:8082/api/v1/managed-accounts',
      },
      {
        source: '/api/v1/orchestrator/:path*',
        destination: 'http://localhost:8090/api/v1/orchestrator/:path*',
      },
      {
        source: '/api/v1/schedules/:path*',
        destination: 'http://localhost:8081/api/v1/schedules/:path*',
      },
    ]
  },
}

module.exports = nextConfig
