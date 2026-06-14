import clearDay from '@meteocons/svg-static/monochrome/clear-day.svg?meteocon';
import clearNight from '@meteocons/svg-static/monochrome/clear-night.svg?meteocon';
import partlyCloudyDay from '@meteocons/svg-static/monochrome/partly-cloudy-day.svg?meteocon';
import partlyCloudyNight from '@meteocons/svg-static/monochrome/partly-cloudy-night.svg?meteocon';
import cloudy from '@meteocons/svg-static/monochrome/cloudy.svg?meteocon';
import overcastDay from '@meteocons/svg-static/monochrome/overcast-day.svg?meteocon';
import overcastNight from '@meteocons/svg-static/monochrome/overcast-night.svg?meteocon';
import drizzle from '@meteocons/svg-static/monochrome/drizzle.svg?meteocon';
import rain from '@meteocons/svg-static/monochrome/rain.svg?meteocon';
import thunderstormsDay from '@meteocons/svg-static/monochrome/thunderstorms-day.svg?meteocon';
import thunderstormsNight from '@meteocons/svg-static/monochrome/thunderstorms-night.svg?meteocon';
import snow from '@meteocons/svg-static/monochrome/snow.svg?meteocon';
import mist from '@meteocons/svg-static/monochrome/mist.svg?meteocon';

// OWM icon-code → Meteocons
export const METEOCONS: Record<string, string> = {
	'01d': clearDay,
	'01n': clearNight,
	'02d': partlyCloudyDay,
	'02n': partlyCloudyNight,
	'03d': cloudy,
	'03n': cloudy,
	'04d': overcastDay,
	'04n': overcastNight,
	'09d': drizzle,
	'09n': drizzle,
	'10d': rain,
	'10n': rain,
	'11d': thunderstormsDay,
	'11n': thunderstormsNight,
	'13d': snow,
	'13n': snow,
	'50d': mist,
	'50n': mist
};

/** Icon for an OWM code; unknown codes fall back to clear-day. */
export function weatherIconFor(code: string): string {
	return METEOCONS[code] ?? clearDay;
}
