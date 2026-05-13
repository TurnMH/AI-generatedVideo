/** @type {import('next').NextConfig} */
const isProdBuild = process.env.NODE_ENV === 'production'
const apiProxyTarget = (process.env.API_PROXY_TARGET || 'http://localhost:8000').replace(/\/+$/, '')

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

const config = {
  // 生产环境改为静态导出，由 Nginx 直接托管产物。
  ...(isProdBuild ? { output: 'export' } : {}),
  trailingSlash: true,
  images: {
    domains: ['localhost', 'cdn.autovideo.ai'],
    unoptimized: true,
  },
  ...devProxyConfig,
}

export default config
