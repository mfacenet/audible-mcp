export interface AudibleMarketplace {
  code: string;
  domain: string;
  marketPlaceId: string;
}

export const audibleMarketplaces: Record<string, AudibleMarketplace> = {
  au: { code: "au", domain: "com.au", marketPlaceId: "AN7EY7DTAW63G" },
  br: { code: "br", domain: "com.br", marketPlaceId: "A10J1VAYUDTYRN" },
  ca: { code: "ca", domain: "ca", marketPlaceId: "A2CQZ5RBY40XE" },
  de: { code: "de", domain: "de", marketPlaceId: "AN7V1F1VY261K" },
  es: { code: "es", domain: "es", marketPlaceId: "ALMIKO4SZCSAR" },
  fr: { code: "fr", domain: "fr", marketPlaceId: "A2728XDNODOQ8T" },
  in: { code: "in", domain: "in", marketPlaceId: "AJO3FBRUE6J4S" },
  it: { code: "it", domain: "it", marketPlaceId: "A2N7FU2W2BU2ZC" },
  jp: { code: "jp", domain: "co.jp", marketPlaceId: "A1QAP3MOU4173J" },
  uk: { code: "uk", domain: "co.uk", marketPlaceId: "A2I9A3Q2GNFNGQ" },
  us: { code: "us", domain: "com", marketPlaceId: "AF2M0KC94RCEA" },
};

export function getMarketplace(code: string): AudibleMarketplace {
  const marketplace = audibleMarketplaces[code.toLowerCase()];
  if (!marketplace) {
    throw new Error(`Unknown Audible marketplace: ${code}`);
  }
  return marketplace;
}

