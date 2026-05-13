/** @type {import('next').NextConfig} */
const isProdBuild = process.env.NODE_ENV === 'production'

const config = {
  // 生产环境改为静态导出，由 Nginx 直接托管产物。
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

    return []
  },
}

export default config
