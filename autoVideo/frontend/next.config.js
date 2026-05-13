/** @type {import('next').NextConfig} */
const apiProxyTarget = (process.env.API_PROXY_TARGET || 'http://localhost:8000').replace(/\/+$/, '')
const isProdBuild = process.env.NODE_ENV === 'production'

const nextConfig = {
  output: 'export',
  trailingSlash: true,
  images: {
    domains: ['localhost', 'cdn.autovideo.ai'],
    unoptimized: true,
  },
  async rewrites() {
    if (isProdBuild) {
      return []
    }

    return [
      {
        source: '/api/:path*',
        destination: `${apiProxyTarget}/api/:path*`,
      },
      {
        source: '/ws/:path*',
        destination: `${apiProxyTarget}/ws/:path*`,
      },
      // /video/* → /projects/* (URL keeps /video, no file duplication)
      {
        source: '/video',
        destination: '/projects',
      },
      {
        source: '/video/:path*',
        destination: '/projects/:path*',
      },
    ]
  },
  webpack: (config, { dev }) => {
    if (dev) {
      config.cache = false
    }
    return config
  },
}

module.exports = nextConfig
