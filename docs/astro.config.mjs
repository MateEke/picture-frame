// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { unified } from '@astrojs/markdown-remark';
import icon from 'astro-icon';
import rehypeExternalLinks from 'rehype-external-links';
import starlightLinksValidator from 'starlight-links-validator';

// GitHub owner/repo (edit + social links) and the deployed docs URL.
// SITE_URL is the Cloudflare Pages URL; update it if the docs move to a custom domain.
const GITHUB_OWNER = 'MateEke';
const REPO_NAME = 'picture-frame';
const SITE_URL = 'https://picture-frame-2kf.pages.dev';

// https://astro.build/config
export default defineConfig({
	// Cloudflare Pages serves at the domain root, so no base path; links stay root-relative.
	site: SITE_URL,
	// Open external links in a new tab, site-wide. Astro 6.4+ takes plugins via a unified()
	// processor; Starlight appends its own transforms to this same processor.
	markdown: {
		processor: unified({
			rehypePlugins: [[rehypeExternalLinks, { target: '_blank', rel: ['noopener', 'noreferrer'] }]]
		})
	},
	integrations: [
		icon(),
		starlight({
			title: 'Picture Frame',
			description:
				'A self-hosted digital picture frame for the Raspberry Pi. No cloud, no telemetry, fully yours.',
			plugins: [starlightLinksValidator()],
			customCss: ['./src/styles/custom.css'],
			favicon: '/favicon.svg',
			// "Edit this page" links (target repo is a placeholder until the repo goes public).
			editLink: {
				baseUrl: `https://github.com/${GITHUB_OWNER}/${REPO_NAME}/edit/main/docs/`
			},
			// Git-based "Last updated" stamps; needs full history in CI (see docs.yml).
			lastUpdated: true,
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: `https://github.com/${GITHUB_OWNER}/${REPO_NAME}`
				}
			],
			components: {
				// Brand lockup (icon + Fraunces wordmark) instead of the plain text title.
				SiteTitle: './src/components/SiteTitle.astro',
				// Adds a subtle "Built with Starlight" credit under the default footer.
				Footer: './src/components/Footer.astro'
			},
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Overview', slug: 'getting-started/overview' },
						{ label: 'Install', slug: 'getting-started/install' },
						{ label: 'Configuration basics', slug: 'getting-started/configuration' }
					]
				},
				{
					label: 'User Manual',
					items: [
						{ label: 'Dashboard', slug: 'manual/dashboard' },
						{ label: 'Photos', slug: 'manual/photos' },
						{ label: 'Slideshow & display', slug: 'manual/slideshow-display' },
						{ label: 'Weather', slug: 'manual/weather' },
						{ label: 'Sensors', slug: 'manual/sensors' },
						{ label: 'Home Assistant', slug: 'manual/home-assistant' },
						{ label: 'Wi-Fi', slug: 'manual/wifi' },
						{ label: 'Software updates', slug: 'manual/updates' },
						{ label: 'Security', slug: 'manual/security' },
						{ label: 'The kiosk display', slug: 'manual/kiosk' }
					]
				},
				{
					label: 'Reference',
					items: [{ label: 'Configuration file', slug: 'reference/configuration' }]
				},
				{
					label: 'Development',
					items: [
						{ label: 'Project status', slug: 'development/status' },
						{ label: 'The story & the hard parts', slug: 'development/story' },
						{ label: 'Contributing', slug: 'development/contributing' }
					]
				}
			]
		})
	]
});
