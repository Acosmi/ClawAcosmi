/** @type {import('next').NextConfig} */
const nextConfig = {
    reactStrictMode: true,
    async rewrites() {
        return [
            {
                source: '/api/sensory/:path*',
                destination: 'http://localhost:8090/api/:path*',
            },
            {
                // VLM chat completions proxy (Go backend)
                source: '/v1/:path*',
                destination: 'http://localhost:8090/v1/:path*',
            },
            {
                // Config management API (Go backend)
                source: '/api/config/:path*',
                destination: 'http://localhost:8090/api/config/:path*',
            },
        ];
    },
};

module.exports = nextConfig;
