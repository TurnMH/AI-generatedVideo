/** @type {import('next').NextConfig} */
const apiProxyTarget = (process.env.API_PROXY_TARGET || 'http://localhost:8000').replace(/\/+$/, '')
const isProdBuild = process.env.NODE_ENV === 'production'

const devProxyConfig = isProdBuild
  ? {}
  : {
      async rewrites() {
        return [
          {
            source: '/api/:path*',
            destination: `${apiProxyTarget}/api/:path*`,
          },
          {
            source: '/ws/:path*',
            destination: `${apiProxyTarget}/ws/:path*`,
          },
        ]
      },
    }

const nextConfig = {
  ...(isProdBuild ? { output: 'export' } : {}),
  trailingSlash: true,
  images: {
    domains: ['localhost', 'cdn.autovideo.ai'],
    unoptimized: true,
  },
  ...devProxyConfig,
  webpack: (config, { dev }) => {
    if (dev) {
      config.cache = false
    }
    return config
  },
}

module.exports = nextConfig
