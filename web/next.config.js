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
        source: '/api/v1/users/import',
        destination: 'http://localhost:8082/api/v1/users/import',
      },
      {
        source: '/api/v1/groups/import',
        destination: 'http://localhost:8082/api/v1/groups/import',
      },
      {
        source: '/api/v1/identity/:path*',
        destination: 'http://localhost:8082/api/v1/identity/:path*',
      },
      {
        source: '/api/v1/users',
        destination: 'http://localhost:8080/api/v1/users',
      },
      {
        source: '/api/v1/users/:path*',
        destination: 'http://localhost:8080/api/v1/users/:path*',
      },
      {
        source: '/api/v1/groups',
        destination: 'http://localhost:8080/api/v1/groups',
      },
      {
        source: '/api/v1/groups/:path*',
        destination: 'http://localhost:8080/api/v1/groups/:path*',
      },
      {
        source: '/api/v1/computers',
        destination: 'http://localhost:8082/api/v1/computers',
      },
      {
        source: '/api/v1/computers/:path*',
        destination: 'http://localhost:8082/api/v1/computers/:path*',
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
        source: '/api/v1/ad-groups',
        destination: 'http://localhost:8082/api/v1/ad-groups',
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
        destination: 'http://localhost:8080/api/v1/schedules/:path*',
      },
      {
        source: '/api/v1/auth/:path*',
        destination: 'http://localhost:8080/api/v1/auth/:path*',
      },
      {
        source: '/api/v1/zones',
        destination: 'http://localhost:8080/api/v1/zones',
      },
      {
        source: '/api/v1/zones/:path*',
        destination: 'http://localhost:8080/api/v1/zones/:path*',
      },
      {
        source: '/api/v1/targets',
        destination: 'http://localhost:8080/api/v1/targets',
      },
      {
        source: '/api/v1/targets/:path*',
        destination: 'http://localhost:8080/api/v1/targets/:path*',
      },
      {
        source: '/api/v1/credentials',
        destination: 'http://localhost:8080/api/v1/credentials',
      },
      {
        source: '/api/v1/credentials/:path*',
        destination: 'http://localhost:8080/api/v1/credentials/:path*',
      },
      {
        source: '/api/v1/audit-logs',
        destination: 'http://localhost:8080/api/v1/audit-logs',
      },
      {
        source: '/api/v1/audit-logs/:path*',
        destination: 'http://localhost:8080/api/v1/audit-logs/:path*',
      },
      {
        source: '/api/v1/system-audit-logs',
        destination: 'http://localhost:8080/api/v1/system-audit-logs',
      },
      {
        source: '/api/v1/system-audit-logs/:path*',
        destination: 'http://localhost:8080/api/v1/system-audit-logs/:path*',
      },
    ]
  },
}

module.exports = nextConfig
