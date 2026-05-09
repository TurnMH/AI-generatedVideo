/** @type {import('next').NextConfig} */
const config = {
  // 生产 Docker 部署所需：输出 standalone 包（包含所有依赖）
  output: 'standalone',
  images: {
    domains: ['localhost', 'cdn.autovideo.ai'],
  },
}

export default config
