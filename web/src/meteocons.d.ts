// tinted Meteocons image source, resolved by the meteocons vite plugin
// (asset URL in builds, data URI on the dev server)
declare module '*.svg?meteocon' {
	const src: string;
	export default src;
}
