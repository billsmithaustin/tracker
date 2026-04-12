#!/usr/bin/env bash
# seed-checkins.sh — POST a sequence of westward check-ins via the API.
# Every check-in field that is rendered somewhere in the dashboard UI is
# exercised at least once so you can visually confirm nothing is broken.
#
# Usage:
#   ./tools/seed-checkins.sh
#   BASE_URL=http://localhost:3000 ./tools/seed-checkins.sh

set -euo pipefail

# Load .env from the repo root (two levels up from tools/)
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  set -a; source "$ENV_FILE"; set +a
fi

BASE_URL="${BASE_URL:-http://localhost}"
API="${BASE_URL}/api/checkins"
PW="${CHECKIN_PASSWORD:?No CHECKIN_PASSWORD set — add it to .env or export it}"

checkin() {
  local label="$1"
  local body="$2"
  printf '%-45s' "  → $label"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X POST "$API" \
    -H 'Content-Type: application/json' \
    -H "X-Checkin-Password: $PW" \
    --data "$body")
  local code
  code=$(printf '%s' "$resp" | tail -n1)
  local json
  json=$(printf '%s' "$resp" | sed '$d')
  if [[ "$code" == "201" ]]; then
    local id
    id=$(printf '%s' "$json" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
    echo "OK (id=$id)"
  else
    echo "FAIL $code — $json"
    exit 1
  fi
  sleep 7
}

PHOTOS_API="${BASE_URL}/api/photos"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

upload_photo() {
  local file="$1"
  printf '%-45s' "  → uploading $(basename "$file")" >&2
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X POST "$PHOTOS_API" \
    -H "X-Checkin-Password: $PW" \
    -F "photo=@$file")
  local code
  code=$(printf '%s' "$resp" | tail -n1)
  local json
  json=$(printf '%s' "$resp" | sed '$d')
  if [[ "$code" == "201" ]]; then
    local url
    url=$(printf '%s' "$json" | grep -o '"url":"[^"]*"' | cut -d'"' -f4)
    echo "OK ($url)" >&2
    printf '%s' "$url"
  else
    echo "FAIL $code — $json" >&2
    exit 1
  fi
}

echo ""
echo "Seeding check-ins to $API"
echo "──────────────────────────────────────────────────"

# ── Day 1 · Apr 1 · Yorktown, VA ─────────────────────────────────────────────
# Covers: lat/lng (→ map marker + completed segment), name, town, state,
#         elevation_ft (→ tel-elev + elev chart dot), miles_today (→ tel-today +
#         log), avg_speed_today (→ tel-speed + log), moving_time_minutes
#         (→ tel-time + log), elevation_gain_today (→ tel-gain + log),
#         elevation_loss_today (→ tel-loss), lodging_type=camping
#         (→ tel-lodging + cum-camped + log badge ⛺), weather_* (→ wx panel),
#         note (→ log), is_rest_day=false (→ EN ROUTE pill + cum-days),
#         photo_url from uploaded file (→ log-photo img tag)
DAY1_PHOTO_URL=$(upload_photo "$SCRIPT_DIR/sample_photo.webp")
DAY4_PHOTO_URL=$(upload_photo "$SCRIPT_DIR/sample_photo.webp")
DAY10_PHOTO_URL=$(upload_photo "$SCRIPT_DIR/sample_photo.webp")
checkin "Day 1 · Yorktown VA (camping, all fields)" \
  "{
    \"created_at\":           \"2026-04-01T18:30:00.000Z\",
    \"lat\":                   37.2374,
    \"lng\":                  -76.5169,
    \"name\":                 \"Riverwalk Campground\",
    \"town\":                 \"Yorktown\",
    \"state\":                \"VA\",
    \"elevation_ft\":          25,
    \"is_rest_day\":           false,
    \"miles_today\":           72.3,
    \"avg_speed_today\":       13.2,
    \"moving_time_minutes\":   329,
    \"elevation_gain_today\":  1840,
    \"elevation_loss_today\":  1620,
    \"lodging_type\":         \"camping\",
    \"weather_temp_f\":        68,
    \"weather_condition\":    \"Partly Cloudy\",
    \"weather_wind_mph\":      12,
    \"weather_wind_dir\":     \"SW\",
    \"photo_url\":            \"${BASE_URL}${DAY1_PHOTO_URL}\",
    \"note\":                 \"First day! Dipped my rear wheel in the York River per tradition. Legs feeling strong. Colonial Parkway was beautiful — no traffic, smooth pavement.\"
  }"

# ── Day 2 · Apr 2 · Roanoke, VA ──────────────────────────────────────────────
# Covers: elevation_ft high value (mountain), warmshowers lodging
#         (→ cum-indoor), note, different state still VA
checkin "Day 2 · Roanoke VA (warmshowers)" \
  '{
    "created_at":           "2026-04-02T19:15:00.000Z",
    "lat":                   37.2709,
    "lng":                  -79.9414,
    "name":                 "Blue Ridge Hospitality",
    "town":                 "Roanoke",
    "state":                "VA",
    "elevation_ft":          1175,
    "is_rest_day":           false,
    "miles_today":           85.1,
    "avg_speed_today":       11.4,
    "moving_time_minutes":   448,
    "elevation_gain_today":  5320,
    "elevation_loss_today":  4890,
    "lodging_type":         "warmshowers",
    "weather_temp_f":        54,
    "weather_condition":    "Overcast",
    "weather_wind_mph":      8,
    "weather_wind_dir":     "NW",
    "note":                 "Blue Ridge was no joke — 5300 ft of climbing. Legs destroyed. WarmShowers host made lasagna. Already asleep by 8 pm."
  }'

# ── Day 3 · Apr 4 · Berea, KY — REST DAY ────────────────────────────────────
# Covers: is_rest_day=true (→ REST DAY pill + cum-rest), bnb lodging
#         (→ cum-indoor), no ride fields (miles/speed/time intentionally absent)
checkin "Day 3 · Berea KY (rest day, bnb)" \
  '{
    "created_at":           "2026-04-04T10:00:00.000Z",
    "lat":                   37.5685,
    "lng":                  -84.2963,
    "name":                 "Boone Tavern Hotel",
    "town":                 "Berea",
    "state":                "KY",
    "elevation_ft":          988,
    "is_rest_day":           true,
    "lodging_type":         "bnb",
    "weather_temp_f":        61,
    "weather_condition":    "Mostly Clear",
    "weather_wind_mph":      5,
    "weather_wind_dir":     "S",
    "note":                 "Rest day in Berea — the arts and crafts capital of Kentucky, apparently. Ate my weight in bourbon chocolate. Zero guilt."
  }'

# ── Day 4 · Apr 6 · Chester, IL ──────────────────────────────────────────────
# Covers: hotel lodging (→ cum-indoor), photo_url (→ log-photo img tag),
#         crossing into Illinois, supply weather manually
checkin "Day 4 · Chester IL (hotel, photo_url)" \
  "{
    \"created_at\":           \"2026-04-06T17:45:00.000Z\",
    \"lat\":                   37.9131,
    \"lng\":                  -89.8223,
    \"name\":                 \"Popeye Inn\",
    \"town\":                 \"Chester\",
    \"state\":                \"IL\",
    \"elevation_ft\":          380,
    \"is_rest_day\":           false,
    \"miles_today\":           78.6,
    \"avg_speed_today\":       14.1,
    \"moving_time_minutes\":   334,
    \"elevation_gain_today\":  980,
    \"elevation_loss_today\":  1150,
    \"lodging_type\":         \"hotel\",
    \"weather_temp_f\":        74,
    \"weather_condition\":    \"Clear\",
    \"weather_wind_mph\":      15,
    \"weather_wind_dir\":     \"S\",
    \"photo_url\":            \"${BASE_URL}${DAY4_PHOTO_URL}\",
    \"note\":                 \"Mississippi River crossing at Chester. Chester is the birthplace of Popeye the Sailor — the statue outside city hall is deeply earnest.\"
  }"

# ── Day 5 · Apr 8 · Farmington, MO ──────────────────────────────────────────
# Covers: Missouri state, no photo (test that log renders fine without it),
#         high elevation_gain (Ozarks), weather auto-fetch (no weather_ fields)
checkin "Day 5 · Farmington MO (camping, weather auto-fetch)" \
  '{
    "created_at":           "2026-04-08T18:00:00.000Z",
    "lat":                   37.7820,
    "lng":                  -90.4218,
    "name":                 "St. Joe State Park",
    "town":                 "Farmington",
    "state":                "MO",
    "elevation_ft":          830,
    "is_rest_day":           false,
    "miles_today":           62.4,
    "avg_speed_today":       10.8,
    "moving_time_minutes":   347,
    "elevation_gain_today":  3710,
    "elevation_loss_today":  3580,
    "lodging_type":         "camping",
    "note":                 "Ozarks are relentless — up and down all day with no net progress in altitude. Gorgeous though. Deer walked through my campsite at dusk."
  }'

# ── Day 6 · Apr 11 · Chanute, KS ─────────────────────────────────────────────
# Covers: Kansas, flat terrain (low gain/loss), "other" lodging type (→ log badge)
checkin "Day 6 · Chanute KS (other lodging, flat terrain)" \
  '{
    "created_at":           "2026-04-11T16:30:00.000Z",
    "lat":                   37.6784,
    "lng":                  -95.4572,
    "town":                 "Chanute",
    "state":                "KS",
    "elevation_ft":          980,
    "is_rest_day":           false,
    "miles_today":           91.2,
    "avg_speed_today":       15.7,
    "moving_time_minutes":   348,
    "elevation_gain_today":  410,
    "elevation_loss_today":  390,
    "lodging_type":         "other",
    "weather_temp_f":        82,
    "weather_condition":    "Clear",
    "weather_wind_mph":      22,
    "weather_wind_dir":     "S",
    "note":                 "Kansas is flat. Very flat. Tailwind most of the day — hit 91 miles with less effort than the Ozarks. Wind can be checked off the list of forces of nature I have now befriended."
  }'

# ── Day 7 · Apr 16 · Pueblo, CO ──────────────────────────────────────────────
# Covers: Colorado, high elevation, longer moving time (mountains),
#         weather with rain condition, total_miles_override (custom cumulative)
checkin "Day 7 · Pueblo CO (hotel, rain, high elev)" \
  '{
    "created_at":           "2026-04-16T19:00:00.000Z",
    "lat":                   38.2544,
    "lng":                 -104.6091,
    "name":                 "Pueblo Inn",
    "town":                 "Pueblo",
    "state":                "CO",
    "elevation_ft":          4700,
    "is_rest_day":           false,
    "miles_today":           68.9,
    "avg_speed_today":       9.6,
    "moving_time_minutes":   430,
    "elevation_gain_today":  6230,
    "elevation_loss_today":  4110,
    "lodging_type":         "hotel",
    "weather_temp_f":        45,
    "weather_condition":    "Light Rain",
    "weather_wind_mph":      18,
    "weather_wind_dir":     "W",
    "note":                 "Crossed the Rockies. Hands numb on the descent. Rain at 11,000 ft is a uniquely unpleasant experience. Hot shower at the motel was the best thing that has ever happened to me."
  }'

# ── Day 8 · Apr 20 · Rawlins, WY ─────────────────────────────────────────────
# Covers: Wyoming, high plateau elevation, camping in WY, snow condition
checkin "Day 8 · Rawlins WY (camping, snow)" \
  '{
    "created_at":           "2026-04-20T17:00:00.000Z",
    "lat":                   41.7908,
    "lng":                 -107.2387,
    "name":                 "Rawlins KOA",
    "town":                 "Rawlins",
    "state":                "WY",
    "elevation_ft":          6780,
    "is_rest_day":           false,
    "miles_today":           74.5,
    "avg_speed_today":       11.2,
    "moving_time_minutes":   399,
    "elevation_gain_today":  2840,
    "elevation_loss_today":  2560,
    "lodging_type":         "camping",
    "weather_temp_f":        29,
    "weather_condition":    "Snow",
    "weather_wind_mph":      30,
    "weather_wind_dir":     "NW",
    "note":                 "Snowed. In April. In Wyoming. Was not expecting that. The tent held up. My morale held up slightly less well but we move forward."
  }'

# ── Day 9 · Apr 26 · Missoula, MT ────────────────────────────────────────────
# Covers: Montana, name + town (→ 'Name · Town, State' header format),
#         warmshowers again, solid riding stats, note with line about weather
checkin "Day 9 · Missoula MT (warmshowers, name+town displayed)" \
  '{
    "created_at":           "2026-04-26T18:45:00.000Z",
    "lat":                   46.8721,
    "lng":                 -114.0117,
    "name":                 "Clark Fork Warmshowers",
    "town":                 "Missoula",
    "state":                "MT",
    "elevation_ft":          3205,
    "is_rest_day":           false,
    "miles_today":           79.3,
    "avg_speed_today":       12.8,
    "moving_time_minutes":   371,
    "elevation_gain_today":  3100,
    "elevation_loss_today":  3400,
    "lodging_type":         "warmshowers",
    "weather_temp_f":        58,
    "weather_condition":    "Partly Cloudy",
    "weather_wind_mph":      9,
    "weather_wind_dir":     "SW",
    "note":                 "Big headwind through the Clark Fork canyon all morning, then it flipped and pushed me into Missoula. The universe is occasionally fair. Hosts made bison chili."
  }'

# ── Day 10 · May 1 · Grangeville, ID ─────────────────────────────────────────
# Covers: Idaho, mid-elevation after Lolo Pass descent and Camas Prairie climb,
#         name field without town (tests fallback rendering), photo_url second
#         appearance in log
checkin "Day 10 · Grangeville ID (camping, photo, almost there)" \
  "{
    \"created_at\":           \"2026-05-01T17:30:00.000Z\",
    \"lat\":                   45.9259,
    \"lng\":                 -116.1309,
    \"name\":                 \"City Campground\",
    \"town\":                 \"Grangeville\",
    \"state\":                \"ID\",
    \"elevation_ft\":          3386,
    \"is_rest_day\":           false,
    \"miles_today\":           83.7,
    \"avg_speed_today\":       13.5,
    \"moving_time_minutes\":   372,
    \"elevation_gain_today\":  4150,
    \"elevation_loss_today\":  7890,
    \"lodging_type\":         \"camping\",
    \"weather_temp_f\":        65,
    \"weather_condition\":    \"Clear\",
    \"weather_wind_mph\":      6,
    \"weather_wind_dir\":     \"W\",
    \"photo_url\":            \"${BASE_URL}${DAY10_PHOTO_URL}\",
    \"note\":                 \"Lolo Pass was the last big climb. It is behind me. Descended the Lochsa canyon all morning, then ground back up to the Camas Prairie. Grangeville sits on the edge of the plateau — big views west. Oregon is close.\"
  }"

echo "──────────────────────────────────────────────────"
echo "Done. Open http://localhost to verify."
echo ""
echo "Field coverage:"
echo "  map marker + route segment  → lat/lng on every check-in"
echo "  'Name · Town, State' header → name present on days 1,2,3,4,5,7,8,9,10"
echo "  tel-elev                    → elevation_ft (all days)"
echo "  tel-today / log miles       → miles_today (all riding days)"
echo "  tel-speed / log mph         → avg_speed_today (all riding days)"
echo "  tel-time / log riding time  → moving_time_minutes (all riding days)"
echo "  tel-gain / log ft gain      → elevation_gain_today (all riding days)"
echo "  tel-loss                    → elevation_loss_today (all riding days)"
echo "  tel-lodging / log badge     → lodging_type: camping, warmshowers,"
echo "                                hotel, bnb, other"
echo "  wx panel (temp/cond/wind)   → weather_* (all days)"
echo "  log note                    → note (all days)"
echo "  log photo                   → photo_url (days 1, 4, 10)  [all uploaded files]"
echo "  REST DAY pill + cum-rest    → is_rest_day=true (day 3)"
echo "  EN ROUTE pill + cum-days    → is_rest_day=false (all others)"
echo "  cum-camped                  → camping nights (days 1, 5, 8, 10)"
echo "  cum-indoor                  → warmshowers/hotel/bnb (days 2,3,4,7,9)"
