# Boverket JSON API with Excel fallback

Boverket publishes Klimatdatabas as open data through Azure APIM (`api-portal.boverket.se` / `api.boverket.se`) and as public Excel files. The JSON path behind APIM is not a stable public URL and may require `BOVERKET_SUBSCRIPTION_KEY`. Ingest therefore treats JSON as preferred: configurable `BOVERKET_API_BASE` (default `https://api.boverket.se`), optional subscription key, and probes of common paths such as `/klimatdatabas/v1/resources`. A 404 or network error must not fail boot.

Fallback is the versioned Excel pair:

- https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-klimatdatabas-version-02.07.000-sv-se.xlsx
- https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-climate-database-version-02.07.000-en-gb.xlsx

Swedish/English workbooks are merged on Resource ID. Typical A1–A3 is the "typiskt värde" / "typical value" column, not the conservative factor. Tests use recorded fixtures; the official workbooks are not committed.
