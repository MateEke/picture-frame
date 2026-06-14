import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
	input: './openapi.json',
	output: './src/lib/api',
	plugins: [
		'@hey-api/typescript',
		{ name: '@hey-api/sdk', operations: { methodName: 'api{{name}}' } },
		{ name: '@hey-api/client-fetch', bundle: true, baseUrl: false }
	]
});
