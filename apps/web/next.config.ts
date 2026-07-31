import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  output: process.env.NEXT_OUTPUT === 'standalone' ? 'standalone' : undefined,
  async rewrites() {
    return [{ source: '/gateway/:path*', destination: 'http://gateway:8080/api/v1/:path*' }];
  },
};

export default nextConfig;
