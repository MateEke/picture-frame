// Every string the on-device touch UI shows. The admin UI stays English; this
// screen is for the people living with the frame.
export const fi = {
	tabs: {
		home: 'Koti',
		gallery: 'Galleria',
		upload: 'Lähetä',
		files: 'Tiedostot',
		settings: 'Asetukset'
	},
	home: {
		startSlideshow: 'Aloita diaesitys',
		screenOff: 'Sammuta näyttö',
		photos: 'kuvaa',
		inSlideshow: 'diaesityksessä',
		files: 'tiedostoa',
		nightWindow: (from: string, until: string) => `Näyttö on yöllä pois päältä ${from}–${until}`,
		idleHint: (after: string) => `Diaesitys alkaa, kun näyttöä ei ole kosketettu ${after}`,
		idleNever: 'Diaesitys alkaa vain napista'
	},
	gallery: {
		empty: 'Ei vielä kuvia. Lähetä kuvia puhelimella Lähetä-välilehdeltä.',
		select: 'Valitse',
		cancel: 'Peruuta',
		selected: (n: number) => (n === 1 ? '1 valittu' : `${n} valittu`),
		selectAll: 'Valitse kaikki',
		show: 'Näytä diaesityksessä',
		hide: 'Piilota diaesityksestä',
		delete: 'Poista',
		hiddenBadge: 'Piilotettu',
		filterAll: 'Kaikki',
		filterShown: 'Diaesityksessä',
		filterHidden: 'Piilotetut',
		confirmDelete: (n: number) =>
			n === 1 ? 'Poistetaanko kuva pysyvästi?' : `Poistetaanko ${n} kuvaa pysyvästi?`,
		deleteFailed: (n: number) => `${n} kuvan poisto epäonnistui`,
		saveFailed: 'Muutoksen tallennus epäonnistui',
		allHiddenNote: 'Kaikki kuvat on piilotettu, joten diaesitys näyttää ne kaikki.',
		close: 'Sulje',
		prev: 'Edellinen',
		next: 'Seuraava',
		immich: 'Kuvat tulevat Immich-albumista, joten niitä ei voi poistaa täältä.'
	},
	upload: {
		title: 'Lähetä kuvia ja tiedostoja',
		steps: [
			'Yhdistä puhelin tai tietokone samaan Wi-Fi-verkkoon.',
			'Skannaa QR-koodi tai avaa osoite selaimessa.',
			'Valitse tai pudota kuvat ja muut tiedostot. Niitä voi lähettää monta kerralla.'
		],
		wifi: 'Verkko',
		noAddress: 'Laitteella ei ole verkkoyhteyttä. Tarkista Wi-Fi hallintasivulta.',
		recent: 'Uusimmat kuvat',
		password: 'Jos hallintasivulle on asetettu salasana, se kysytään ensin.'
	},
	files: {
		empty: 'Ei tiedostoja. Muut kuin kuvatiedostot (videot, PDF:t, asiakirjat) näkyvät täällä.',
		free: (free: string) => `Vapaata tilaa ${free}`,
		delete: 'Poista',
		confirmDelete: (name: string) => `Poistetaanko ${name} pysyvästi?`,
		deleteFailed: 'Poisto epäonnistui',
		noPreview: 'Tätä tiedostoa ei voi esikatsella näytöllä.',
		openOnPhone: 'Avaa puhelimella skannaamalla koodi:',
		loading: 'Ladataan…',
		textTruncated: '… (näytetään alku)',
		close: 'Sulje'
	},
	settings: {
		saved: 'Tallennettu',
		saving: 'Tallennetaan…',
		saveFailed: 'Tallennus epäonnistui',
		loadFailed: 'Asetuksia ei voitu ladata.',
		slideshow: 'Diaesitys',
		interval: 'Kuvanvaihto',
		randomize: 'Satunnainen järjestys',
		split: 'Pystykuvat rinnakkain',
		splitHint: 'Kaksi samansuuntaista kuvaa vierekkäin rajaamisen sijaan',
		sleep: 'Lepotila',
		idleAfter: 'Diaesitys alkaa, kun näyttöä ei ole kosketettu',
		night: 'Yöaikataulu',
		nightEnabled: 'Sammuta näyttö yöksi',
		offFrom: 'Sammuu klo',
		offUntil: 'Syttyy klo',
		wakeFor: 'Kosketus sytyttää yöllä ajaksi',
		screen: 'Näyttö',
		brightness: 'Kirkkaus',
		brightnessUnset: 'Ei säädetä',
		rotation: 'Kierto',
		about: 'Tietoja',
		address: 'Osoite',
		network: 'Wi-Fi',
		version: 'Versio',
		adminHint: 'Muut asetukset (Wi-Fi, sää, anturit, päivitykset) löytyvät hallintasivulta.'
	},
	wakeHint: 'Kosketa herättääksesi'
} as const;
