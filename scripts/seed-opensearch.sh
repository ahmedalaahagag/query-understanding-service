#!/usr/bin/env bash
# seed-opensearch.sh — Seeds local OpenSearch with per-locale HelloFresh food concepts and linguistic data.
# Supports 26 locales across all HelloFresh markets.
# Inline data: en_gb, en_us, en_ca, fr_ca, de_de
# File-based data (locale-data/): all other locales
# Built from real HelloFresh recipe catalogues across all markets.
# Usage: ./scripts/seed-opensearch.sh [OPENSEARCH_URL]
set -euo pipefail

OS_URL="${1:-http://localhost:9200}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOCALE_DATA_DIR="${SCRIPT_DIR}/locale-data"

# All supported locales
ALL_LOCALES="en_gb en_us en_ca fr_ca de_de da_dk de_at de_ch en_au en_be en_de en_dk en_ie en_nl en_nz en_se es_es fr_be fr_fr fr_lu it_it ja_jp nb_no nl_be nl_nl sv_se"

echo "==> Seeding OpenSearch at ${OS_URL}"

# ---------------------------------------------------------------------------
# Helper: bulk index from a file to avoid shell argument limits
# ---------------------------------------------------------------------------
bulk_index() {
  local index="$1"
  local file="$2"
  curl -s -X POST "${OS_URL}/${index}/_bulk" \
    -H 'Content-Type: application/x-ndjson' \
    --data-binary "@${file}" | jq -r '"  '${index}': \(.items | length) docs, errors=\(.errors)"'
}

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# ---------------------------------------------------------------------------
# 1. Delete existing indices (ignore 404)
# ---------------------------------------------------------------------------
echo "==> Deleting existing indices..."
for locale in ${ALL_LOCALES}; do
  curl -s -X DELETE "${OS_URL}/concepts_${locale}" -o /dev/null || true
  curl -s -X DELETE "${OS_URL}/linguistic_${locale}" -o /dev/null || true
done

# ---------------------------------------------------------------------------
# 2. Create per-locale concept indices
# ---------------------------------------------------------------------------
CONCEPT_MAPPING='{
  "settings": { "number_of_shards": 1, "number_of_replicas": 0 },
  "mappings": {
    "properties": {
      "id":      { "type": "keyword" },
      "label":   { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "field":   { "type": "keyword" },
      "aliases": { "type": "text" },
      "weight":  { "type": "integer" },
      "locale":  { "type": "keyword" },
      "market":  { "type": "keyword" }
    }
  }
}'

LINGUISTIC_MAPPING='{
  "settings": { "number_of_shards": 1, "number_of_replicas": 0 },
  "mappings": {
    "properties": {
      "term":    { "type": "keyword" },
      "variant": { "type": "keyword" },
      "type":    { "type": "keyword" },
      "locale":  { "type": "keyword" }
    }
  }
}'

echo "==> Creating per-locale indices..."
for locale in ${ALL_LOCALES}; do
  curl -s -X PUT "${OS_URL}/concepts_${locale}" -H 'Content-Type: application/json' -d "${CONCEPT_MAPPING}" | jq -r '"  concepts_'"${locale}"': acknowledged=\(.acknowledged)"'
  curl -s -X PUT "${OS_URL}/linguistic_${locale}" -H 'Content-Type: application/json' -d "${LINGUISTIC_MAPPING}" | jq -r '"  linguistic_'"${locale}"': acknowledged=\(.acknowledged)"'
done

# ============================================================================
# 4. CONCEPTS — Full HelloFresh food taxonomy
#    Sources: hellofresh.co.uk/recipes, popular, vegetarian, healthy pages
#    Fields: category, cuisine, dietary, ingredient, meal_type, cooking_method
# ============================================================================
echo "==> Indexing GB concepts..."

cat > "${TMPDIR}/concepts_en_gb.ndjson" << 'NDJSON'
{"index":{"_id":"cat-chicken"}}
{"id":"cat-chicken","label":"chicken","field":"category","aliases":["poultry","chook","whole chicken"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chicken-breast"}}
{"id":"cat-chicken-breast","label":"chicken breast","field":"category","aliases":["breast of chicken","chicken fillet","chicken escalope"],"weight":12,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chicken-thigh"}}
{"id":"cat-chicken-thigh","label":"chicken thigh","field":"category","aliases":["thigh fillet","boneless thigh","chicken thighs"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chicken-drumstick"}}
{"id":"cat-chicken-drumstick","label":"chicken drumstick","field":"category","aliases":["drumstick","chicken leg","chicken drumsticks"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chicken-wing"}}
{"id":"cat-chicken-wing","label":"chicken wing","field":"category","aliases":["chicken wings","buffalo wings","hot wings"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-beef"}}
{"id":"cat-beef","label":"beef","field":"category","aliases":["cow","bovine"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-minced-beef"}}
{"id":"cat-minced-beef","label":"minced beef","field":"category","aliases":["ground beef","beef mince","mince"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-beef-steak"}}
{"id":"cat-beef-steak","label":"beef steak","field":"category","aliases":["sirloin","sirloin steak","ribeye","fillet steak","21 day aged sirloin"],"weight":12,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-beef-meatball"}}
{"id":"cat-beef-meatball","label":"beef meatballs","field":"category","aliases":["meatballs","beef meatball"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pork"}}
{"id":"cat-pork","label":"pork","field":"category","aliases":["pig"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pork-steak"}}
{"id":"cat-pork-steak","label":"pork steak","field":"category","aliases":["pork steaks","pork loin steak"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pork-chop"}}
{"id":"cat-pork-chop","label":"pork chop","field":"category","aliases":["chop","pork cutlet","pork chops"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pulled-pork"}}
{"id":"cat-pulled-pork","label":"pulled pork","field":"category","aliases":["slow cooked pork","shredded pork"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pork-meatball"}}
{"id":"cat-pork-meatball","label":"pork meatballs","field":"category","aliases":["pork meatball"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-bacon"}}
{"id":"cat-bacon","label":"bacon","field":"category","aliases":["streaky bacon","back bacon","smoked bacon","bacon rashers"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chorizo"}}
{"id":"cat-chorizo","label":"chorizo","field":"category","aliases":["spanish sausage","double chorizo","chorizo sausage"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sausage"}}
{"id":"cat-sausage","label":"sausage","field":"category","aliases":["banger","pork sausage","sausages","cumberland sausage","cumberland sausages"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-serrano-ham"}}
{"id":"cat-serrano-ham","label":"serrano ham","field":"category","aliases":["spanish ham","jamon","cured ham"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-gammon"}}
{"id":"cat-gammon","label":"gammon","field":"category","aliases":["gammon steak","ham steak"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lamb"}}
{"id":"cat-lamb","label":"lamb","field":"category","aliases":["mutton"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lamb-mince"}}
{"id":"cat-lamb-mince","label":"lamb mince","field":"category","aliases":["minced lamb","ground lamb"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lamb-steak"}}
{"id":"cat-lamb-steak","label":"lamb steak","field":"category","aliases":["lamb leg steak","lamb steaks"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lamb-meatball"}}
{"id":"cat-lamb-meatball","label":"lamb meatballs","field":"category","aliases":["herby lamb meatballs","lamb meatball"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-duck"}}
{"id":"cat-duck","label":"duck","field":"category","aliases":["duck leg","confit duck","crispy duck","duck breast"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-salmon"}}
{"id":"cat-salmon","label":"salmon","field":"category","aliases":["salmon fillet","atlantic salmon","salmon fillets"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cod"}}
{"id":"cat-cod","label":"cod","field":"category","aliases":["cod fillet","atlantic cod","cod fillets"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sea-bream"}}
{"id":"cat-sea-bream","label":"sea bream","field":"category","aliases":["bream","sea bream fillet"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-basa"}}
{"id":"cat-basa","label":"basa","field":"category","aliases":["basa fillet","white fish","basa fish"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-barramundi"}}
{"id":"cat-barramundi","label":"barramundi","field":"category","aliases":["barramundi fillet","asian sea bass"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-prawns"}}
{"id":"cat-prawns","label":"prawns","field":"category","aliases":["shrimp","king prawns","tiger prawns","double prawns"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-crab"}}
{"id":"cat-crab","label":"crab","field":"category","aliases":["orkney crab","crab meat","dressed crab"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-tuna"}}
{"id":"cat-tuna","label":"tuna","field":"category","aliases":["tuna steak","ahi tuna","tinned tuna"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-smoked-fish"}}
{"id":"cat-smoked-fish","label":"smoked fish","field":"category","aliases":["smoked haddock","smoked salmon","kipper"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-tofu"}}
{"id":"cat-tofu","label":"tofu","field":"category","aliases":["bean curd","soybean curd","double tofu","firm tofu"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-halloumi"}}
{"id":"cat-halloumi","label":"halloumi","field":"category","aliases":["halloumi cheese","grilling cheese","double halloumi","halloumi steaks"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-paneer"}}
{"id":"cat-paneer","label":"paneer","field":"category","aliases":["indian cheese","chilli paneer","double paneer"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-falafel"}}
{"id":"cat-falafel","label":"falafel","field":"category","aliases":["double falafel","chickpea falafel"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-gyoza"}}
{"id":"cat-gyoza","label":"gyoza","field":"category","aliases":["veggie gyoza","vegetable gyoza","dumplings","potstickers"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-plant-based"}}
{"id":"cat-plant-based","label":"plant based","field":"category","aliases":["meat free","this isnt chicken","this isnt pork","meat substitute","meat alternative"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-egg"}}
{"id":"cat-egg","label":"egg","field":"ingredient","aliases":["eggs","free range egg","fried egg","poached egg"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pasta"}}
{"id":"cat-pasta","label":"pasta","field":"category","aliases":["italian pasta","dried pasta"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-penne"}}
{"id":"cat-penne","label":"penne","field":"category","aliases":["penne pasta","penne rigate"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-spaghetti"}}
{"id":"cat-spaghetti","label":"spaghetti","field":"category","aliases":["spaghetti pasta"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-fusilli"}}
{"id":"cat-fusilli","label":"fusilli","field":"category","aliases":["fusilli pasta","spiral pasta","twists"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-linguine"}}
{"id":"cat-linguine","label":"linguine","field":"category","aliases":["linguine pasta"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-orzo"}}
{"id":"cat-orzo","label":"orzo","field":"category","aliases":["orzo pasta","risoni"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-rigatoni"}}
{"id":"cat-rigatoni","label":"rigatoni","field":"category","aliases":["rigatoni pasta"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-macaroni"}}
{"id":"cat-macaroni","label":"macaroni","field":"category","aliases":["macaroni pasta","mac","elbow pasta"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-ravioli"}}
{"id":"cat-ravioli","label":"ravioli","field":"category","aliases":["ricotta ravioli","filled pasta"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-gnocchi"}}
{"id":"cat-gnocchi","label":"gnocchi","field":"category","aliases":["potato gnocchi","italian gnocchi"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-ditali"}}
{"id":"cat-ditali","label":"ditali","field":"category","aliases":["ditali pasta","short pasta"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-rice"}}
{"id":"cat-rice","label":"rice","field":"category","aliases":["white rice","long grain rice"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-basmati"}}
{"id":"cat-basmati","label":"basmati rice","field":"category","aliases":["basmati","aromatic rice"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-jasmine-rice"}}
{"id":"cat-jasmine-rice","label":"jasmine rice","field":"category","aliases":["jasmine","thai rice","fragrant rice"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sticky-rice"}}
{"id":"cat-sticky-rice","label":"sticky rice","field":"category","aliases":["glutinous rice","sushi rice"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-noodles"}}
{"id":"cat-noodles","label":"noodles","field":"category","aliases":["egg noodles","rice noodles"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-udon"}}
{"id":"cat-udon","label":"udon","field":"category","aliases":["udon noodles","thick noodles"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-vermicelli"}}
{"id":"cat-vermicelli","label":"vermicelli","field":"category","aliases":["vermicelli noodles","rice vermicelli","thin noodles"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-couscous"}}
{"id":"cat-couscous","label":"couscous","field":"category","aliases":["moroccan couscous","giant couscous"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-bulgur"}}
{"id":"cat-bulgur","label":"bulgur wheat","field":"category","aliases":["bulgur","cracked wheat","bulghur"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lentils"}}
{"id":"cat-lentils","label":"lentils","field":"category","aliases":["red lentils","green lentils","puy lentils"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chickpeas"}}
{"id":"cat-chickpeas","label":"chickpeas","field":"category","aliases":["garbanzo beans","tinned chickpeas","roasted chickpeas"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-butter-beans"}}
{"id":"cat-butter-beans","label":"butter beans","field":"category","aliases":["lima beans","tinned butter beans"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-kidney-beans"}}
{"id":"cat-kidney-beans","label":"kidney beans","field":"category","aliases":["red kidney beans","tinned kidney beans"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-potato"}}
{"id":"cat-potato","label":"potato","field":"ingredient","aliases":["potatoes","spud","spuds","maris piper","roast potatoes","mashed potato","dauphinoise"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sweet-potato"}}
{"id":"cat-sweet-potato","label":"sweet potato","field":"ingredient","aliases":["sweet potatoes","yam","sweet potato wedges"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chips"}}
{"id":"cat-chips","label":"chips","field":"ingredient","aliases":["fries","french fries","seasoned chips","cheesy chips","wedges"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-broccoli"}}
{"id":"cat-broccoli","label":"broccoli","field":"ingredient","aliases":["tenderstem broccoli","broccoli florets","long stem broccoli"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cauliflower"}}
{"id":"cat-cauliflower","label":"cauliflower","field":"ingredient","aliases":["cauliflower florets","roasted cauliflower"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pepper"}}
{"id":"cat-pepper","label":"pepper","field":"ingredient","aliases":["bell pepper","capsicum","red pepper","green pepper","yellow pepper","mixed peppers"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-onion"}}
{"id":"cat-onion","label":"onion","field":"ingredient","aliases":["onions","red onion","white onion","brown onion","caramelised onion","spring onion"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-garlic"}}
{"id":"cat-garlic","label":"garlic","field":"ingredient","aliases":["garlic clove","garlic cloves","minced garlic","garlic paste"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-tomato"}}
{"id":"cat-tomato","label":"tomato","field":"ingredient","aliases":["tomatoes","cherry tomato","chopped tomatoes","tinned tomatoes","baby plum tomatoes","sun dried tomatoes"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-mushroom"}}
{"id":"cat-mushroom","label":"mushroom","field":"ingredient","aliases":["mushrooms","chestnut mushroom","portobello","shiitake","truffle mushroom"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-courgette"}}
{"id":"cat-courgette","label":"courgette","field":"ingredient","aliases":["zucchini","baby marrow","courgettes"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-aubergine"}}
{"id":"cat-aubergine","label":"aubergine","field":"ingredient","aliases":["eggplant","brinjal","aubergines"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-spinach"}}
{"id":"cat-spinach","label":"spinach","field":"ingredient","aliases":["baby spinach","spinach leaves"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pak-choi"}}
{"id":"cat-pak-choi","label":"pak choi","field":"ingredient","aliases":["bok choy","bok choi","chinese cabbage"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cabbage"}}
{"id":"cat-cabbage","label":"cabbage","field":"ingredient","aliases":["red cabbage","white cabbage","savoy cabbage","spring cabbage"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-kale"}}
{"id":"cat-kale","label":"kale","field":"ingredient","aliases":["curly kale","cavolo nero","black kale"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-leek"}}
{"id":"cat-leek","label":"leek","field":"ingredient","aliases":["leeks","baby leek"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-carrot"}}
{"id":"cat-carrot","label":"carrot","field":"ingredient","aliases":["carrots","baby carrots"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-parsnip"}}
{"id":"cat-parsnip","label":"parsnip","field":"ingredient","aliases":["parsnips","roasted parsnips"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-beetroot"}}
{"id":"cat-beetroot","label":"beetroot","field":"ingredient","aliases":["beet","beets","red beet"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-butternut-squash"}}
{"id":"cat-butternut-squash","label":"butternut squash","field":"ingredient","aliases":["squash","butternut","roasted squash"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-fennel"}}
{"id":"cat-fennel","label":"fennel","field":"ingredient","aliases":["fennel bulb"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-asparagus"}}
{"id":"cat-asparagus","label":"asparagus","field":"ingredient","aliases":["asparagus spears","asparagus tips"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-green-beans"}}
{"id":"cat-green-beans","label":"green beans","field":"ingredient","aliases":["fine beans","french beans","runner beans"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-peas"}}
{"id":"cat-peas","label":"peas","field":"ingredient","aliases":["garden peas","petit pois","young pea pods","sugar snap peas","mange tout"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sweetcorn"}}
{"id":"cat-sweetcorn","label":"sweetcorn","field":"ingredient","aliases":["corn","corn on the cob","baby corn","charred corn"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cucumber"}}
{"id":"cat-cucumber","label":"cucumber","field":"ingredient","aliases":["smacked cucumber","pickled cucumber"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-rocket"}}
{"id":"cat-rocket","label":"rocket","field":"ingredient","aliases":["arugula","wild rocket","rocket salad"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lettuce"}}
{"id":"cat-lettuce","label":"lettuce","field":"ingredient","aliases":["baby gem","baby gem lettuce","iceberg","little gem","romaine"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-radish"}}
{"id":"cat-radish","label":"radish","field":"ingredient","aliases":["radishes","pickled radish"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-avocado"}}
{"id":"cat-avocado","label":"avocado","field":"ingredient","aliases":["avo","avocados","guacamole"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-brussels-sprouts"}}
{"id":"cat-brussels-sprouts","label":"brussels sprouts","field":"ingredient","aliases":["sprouts","brussel sprouts"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pomegranate"}}
{"id":"cat-pomegranate","label":"pomegranate","field":"ingredient","aliases":["pomegranate seeds"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pineapple"}}
{"id":"cat-pineapple","label":"pineapple","field":"ingredient","aliases":["pineapple chunks","pineapple relish"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cheese"}}
{"id":"cat-cheese","label":"cheese","field":"ingredient","aliases":["grated cheese"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cheddar"}}
{"id":"cat-cheddar","label":"cheddar","field":"ingredient","aliases":["cheddar cheese","mature cheddar","grated cheddar"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-mozzarella"}}
{"id":"cat-mozzarella","label":"mozzarella","field":"ingredient","aliases":["mozzarella cheese","fresh mozzarella"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-parmesan"}}
{"id":"cat-parmesan","label":"parmesan","field":"ingredient","aliases":["parmigiano reggiano","parmigiano","italian hard cheese","grana padano"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-burrata"}}
{"id":"cat-burrata","label":"burrata","field":"ingredient","aliases":["burrata cheese","fresh burrata"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-goats-cheese"}}
{"id":"cat-goats-cheese","label":"goats cheese","field":"ingredient","aliases":["goat cheese","chevre"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-ricotta"}}
{"id":"cat-ricotta","label":"ricotta","field":"ingredient","aliases":["ricotta cheese"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-blue-cheese"}}
{"id":"cat-blue-cheese","label":"blue cheese","field":"ingredient","aliases":["stilton","gorgonzola","roquefort"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-feta"}}
{"id":"cat-feta","label":"feta","field":"ingredient","aliases":["feta cheese","greek style cheese","greek cheese"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cream"}}
{"id":"cat-cream","label":"cream","field":"ingredient","aliases":["double cream","single cream","heavy cream","soured cream","creme fraiche"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-coconut-milk"}}
{"id":"cat-coconut-milk","label":"coconut milk","field":"ingredient","aliases":["coconut cream","tinned coconut milk"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-yoghurt"}}
{"id":"cat-yoghurt","label":"yoghurt","field":"ingredient","aliases":["yogurt","greek yoghurt","natural yoghurt","greek style yoghurt"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-butter"}}
{"id":"cat-butter","label":"butter","field":"ingredient","aliases":["unsalted butter","salted butter","paprika butter"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-olive-oil"}}
{"id":"cat-olive-oil","label":"olive oil","field":"ingredient","aliases":["extra virgin olive oil","evoo"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-flour"}}
{"id":"cat-flour","label":"flour","field":"ingredient","aliases":["plain flour","self raising flour","bread flour"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-tortilla"}}
{"id":"cat-tortilla","label":"tortilla","field":"ingredient","aliases":["wrap","flour tortilla","corn tortilla","tortillas","soft tortilla"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-bread"}}
{"id":"cat-bread","label":"bread","field":"ingredient","aliases":["sourdough","ciabatta","baguette","bread roll","flatbread","naan","brioche","pitta","focaccia"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-puff-pastry"}}
{"id":"cat-puff-pastry","label":"puff pastry","field":"ingredient","aliases":["pastry","puff pastry sheet","shortcrust pastry"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-filo-pastry"}}
{"id":"cat-filo-pastry","label":"filo pastry","field":"ingredient","aliases":["filo","phyllo","filo sheets"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-panko"}}
{"id":"cat-panko","label":"panko breadcrumbs","field":"ingredient","aliases":["panko","breadcrumbs","crispy coating"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-soy-sauce"}}
{"id":"cat-soy-sauce","label":"soy sauce","field":"ingredient","aliases":["soya sauce","shoyu","tamari","dark soy sauce","light soy sauce"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-ginger"}}
{"id":"cat-ginger","label":"ginger","field":"ingredient","aliases":["fresh ginger","ginger root","ground ginger","ginger paste"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chilli"}}
{"id":"cat-chilli","label":"chilli","field":"ingredient","aliases":["chili","red chilli","green chilli","chilli pepper","hot pepper","chilli flakes"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cumin"}}
{"id":"cat-cumin","label":"cumin","field":"ingredient","aliases":["ground cumin","cumin seeds","jeera"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-paprika"}}
{"id":"cat-paprika","label":"paprika","field":"ingredient","aliases":["smoked paprika","sweet paprika","hot paprika"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-coriander"}}
{"id":"cat-coriander","label":"coriander","field":"ingredient","aliases":["cilantro","fresh coriander","ground coriander","coriander seeds"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-garam-masala"}}
{"id":"cat-garam-masala","label":"garam masala","field":"ingredient","aliases":["indian spice mix","curry powder"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-turmeric"}}
{"id":"cat-turmeric","label":"turmeric","field":"ingredient","aliases":["ground turmeric","haldi"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-mustard-seeds"}}
{"id":"cat-mustard-seeds","label":"mustard seeds","field":"ingredient","aliases":["yellow mustard seeds","brown mustard seeds"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-thyme"}}
{"id":"cat-thyme","label":"thyme","field":"ingredient","aliases":["fresh thyme","dried thyme"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-rosemary"}}
{"id":"cat-rosemary","label":"rosemary","field":"ingredient","aliases":["fresh rosemary","dried rosemary"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sage"}}
{"id":"cat-sage","label":"sage","field":"ingredient","aliases":["fresh sage","sage leaves"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-parsley"}}
{"id":"cat-parsley","label":"parsley","field":"ingredient","aliases":["fresh parsley","flat leaf parsley","curly parsley"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-basil"}}
{"id":"cat-basil","label":"basil","field":"ingredient","aliases":["fresh basil","basil leaves","thai basil"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-dill"}}
{"id":"cat-dill","label":"dill","field":"ingredient","aliases":["fresh dill","dill weed"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-tarragon"}}
{"id":"cat-tarragon","label":"tarragon","field":"ingredient","aliases":["fresh tarragon"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chives"}}
{"id":"cat-chives","label":"chives","field":"ingredient","aliases":["fresh chives","snipped chives"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lemon"}}
{"id":"cat-lemon","label":"lemon","field":"ingredient","aliases":["lemon juice","lemon zest","lemons"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-lime"}}
{"id":"cat-lime","label":"lime","field":"ingredient","aliases":["lime juice","lime zest","limes"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-honey"}}
{"id":"cat-honey","label":"honey","field":"ingredient","aliases":["runny honey","clear honey","hot honey"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-passata"}}
{"id":"cat-passata","label":"passata","field":"ingredient","aliases":["tomato passata","sieved tomatoes"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pesto"}}
{"id":"cat-pesto","label":"pesto","field":"ingredient","aliases":["green pesto","basil pesto","red pesto","rocket pesto"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-mayo"}}
{"id":"cat-mayo","label":"mayonnaise","field":"ingredient","aliases":["mayo","garlic mayo","aioli","sambal mayo","lemon mayo"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-stock"}}
{"id":"cat-stock","label":"stock","field":"ingredient","aliases":["chicken stock","beef stock","vegetable stock","stock cube","bouillon"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-sriracha"}}
{"id":"cat-sriracha","label":"sriracha","field":"ingredient","aliases":["hot sauce","chilli sauce","sriracha sauce"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-teriyaki"}}
{"id":"cat-teriyaki","label":"teriyaki","field":"ingredient","aliases":["teriyaki sauce","teriyaki glaze"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-hoisin"}}
{"id":"cat-hoisin","label":"hoisin","field":"ingredient","aliases":["hoisin sauce","peking sauce","ginger hoisin"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-harissa"}}
{"id":"cat-harissa","label":"harissa","field":"ingredient","aliases":["harissa paste","rose harissa","chermoula harissa"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-gochujang"}}
{"id":"cat-gochujang","label":"gochujang","field":"ingredient","aliases":["korean chilli paste","gochujang paste","honey gochujang"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-miso"}}
{"id":"cat-miso","label":"miso","field":"ingredient","aliases":["miso paste","white miso","red miso"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-buffalo-sauce"}}
{"id":"cat-buffalo-sauce","label":"buffalo sauce","field":"ingredient","aliases":["buffalo","franks hot sauce"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-satay"}}
{"id":"cat-satay","label":"satay","field":"ingredient","aliases":["satay sauce","peanut sauce","satay dip"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cajun"}}
{"id":"cat-cajun","label":"cajun","field":"ingredient","aliases":["cajun spice","cajun seasoning","cajun spiced"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-jerk"}}
{"id":"cat-jerk","label":"jerk","field":"ingredient","aliases":["jerk seasoning","jerk spice","jerk marinade","caribbean jerk"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-chermoula"}}
{"id":"cat-chermoula","label":"chermoula","field":"ingredient","aliases":["chermoula paste","moroccan chermoula"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-nduja"}}
{"id":"cat-nduja","label":"nduja","field":"ingredient","aliases":["nduja paste","veggie nduja","calabrian nduja"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-tahini"}}
{"id":"cat-tahini","label":"tahini","field":"ingredient","aliases":["tahini paste","sesame paste"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-houmous"}}
{"id":"cat-houmous","label":"houmous","field":"ingredient","aliases":["hummus","humous","chickpea dip"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-mango-chutney"}}
{"id":"cat-mango-chutney","label":"mango chutney","field":"ingredient","aliases":["chutney","sweet chutney"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-balsamic"}}
{"id":"cat-balsamic","label":"balsamic","field":"ingredient","aliases":["balsamic vinegar","balsamic glaze","balsamic reduction"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-truffle"}}
{"id":"cat-truffle","label":"truffle","field":"ingredient","aliases":["truffle oil","truffle sauce","truffled"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-peanuts"}}
{"id":"cat-peanuts","label":"peanuts","field":"ingredient","aliases":["peanut","roasted peanuts","crushed peanuts"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-almonds"}}
{"id":"cat-almonds","label":"almonds","field":"ingredient","aliases":["flaked almonds","almond","ground almonds"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-pine-nuts"}}
{"id":"cat-pine-nuts","label":"pine nuts","field":"ingredient","aliases":["pine kernels","pignoli"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-hazelnuts"}}
{"id":"cat-hazelnuts","label":"hazelnuts","field":"ingredient","aliases":["hazelnut","toasted hazelnuts"],"weight":6,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cat-cranberries"}}
{"id":"cat-cranberries","label":"cranberries","field":"ingredient","aliases":["dried cranberries","cranberry"],"weight":5,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-burger"}}
{"id":"mt-burger","label":"burger","field":"meal_type","aliases":["hamburger","beef burger","cheeseburger","chicken burger","fried chicken burger"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-stir-fry"}}
{"id":"mt-stir-fry","label":"stir fry","field":"meal_type","aliases":["stirfry","wok","noodle stir fry"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-curry"}}
{"id":"mt-curry","label":"curry","field":"meal_type","aliases":["indian curry","thai curry","coconut curry"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-tikka-masala"}}
{"id":"mt-tikka-masala","label":"tikka masala","field":"meal_type","aliases":["chicken tikka masala","tikka"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-korma"}}
{"id":"mt-korma","label":"korma","field":"meal_type","aliases":["chicken korma","mild curry"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-thai-green-curry"}}
{"id":"mt-thai-green-curry","label":"thai green curry","field":"meal_type","aliases":["green curry","thai curry","red thai curry"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-dal"}}
{"id":"mt-dal","label":"dal","field":"meal_type","aliases":["dhal","daal","lentil curry","lentil dal"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-bolognese"}}
{"id":"mt-bolognese","label":"bolognese","field":"meal_type","aliases":["spag bol","spaghetti bolognese","ragu","meat sauce"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-carbonara"}}
{"id":"mt-carbonara","label":"carbonara","field":"meal_type","aliases":["spaghetti carbonara","pasta carbonara"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-alfredo"}}
{"id":"mt-alfredo","label":"alfredo","field":"meal_type","aliases":["pasta alfredo","chicken alfredo","creamy alfredo"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-lasagne"}}
{"id":"mt-lasagne","label":"lasagne","field":"meal_type","aliases":["lasagna","beef lasagne","veggie lasagne"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-risotto"}}
{"id":"mt-risotto","label":"risotto","field":"meal_type","aliases":["italian risotto","mushroom risotto","oven baked risotto"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-paella"}}
{"id":"mt-paella","label":"paella","field":"meal_type","aliases":["spanish paella","seafood paella"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-pie"}}
{"id":"mt-pie","label":"pie","field":"meal_type","aliases":["chicken pie","beef pie","pastry pie","puff pastry pie"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-cottage-pie"}}
{"id":"mt-cottage-pie","label":"cottage pie","field":"meal_type","aliases":["beef cottage pie"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-shepherds-pie"}}
{"id":"mt-shepherds-pie","label":"shepherds pie","field":"meal_type","aliases":["shepherd pie","lamb shepherds pie"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-soup"}}
{"id":"mt-soup","label":"soup","field":"meal_type","aliases":["broth","winter soup","ribollita"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-stew"}}
{"id":"mt-stew","label":"stew","field":"meal_type","aliases":["winter stew","hearty stew"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-casserole"}}
{"id":"mt-casserole","label":"casserole","field":"meal_type","aliases":["one pot","hotpot","winter warmer"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-chilli"}}
{"id":"mt-chilli","label":"chilli","field":"meal_type","aliases":["chilli con carne","chili","chilli non carne","beef chilli","bean chilli"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-salad"}}
{"id":"mt-salad","label":"salad","field":"meal_type","aliases":["side salad","green salad","mixed salad","warm salad"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-wrap"}}
{"id":"mt-wrap","label":"wrap","field":"meal_type","aliases":["chicken wrap","tortilla wrap"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-tacos"}}
{"id":"mt-tacos","label":"tacos","field":"meal_type","aliases":["taco","soft tacos","crispy tacos","fish tacos","chicken tacos","duck tacos"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-burrito"}}
{"id":"mt-burrito","label":"burrito","field":"meal_type","aliases":["bean burrito","chicken burrito","burrito bowl"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-fajitas"}}
{"id":"mt-fajitas","label":"fajitas","field":"meal_type","aliases":["fajita","chicken fajitas"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-enchiladas"}}
{"id":"mt-enchiladas","label":"enchiladas","field":"meal_type","aliases":["enchilada","chicken enchiladas"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-quesadilla"}}
{"id":"mt-quesadilla","label":"quesadilla","field":"meal_type","aliases":["quesadillas","cheese quesadilla","mushroom quesadilla"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-nachos"}}
{"id":"mt-nachos","label":"nachos","field":"meal_type","aliases":["loaded nachos","cheesy nachos"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-pizza"}}
{"id":"mt-pizza","label":"pizza","field":"meal_type","aliases":["flatbread pizza","homemade pizza","naanizza"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-calzone"}}
{"id":"mt-calzone","label":"calzone","field":"meal_type","aliases":["folded pizza","stuffed pizza"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-sandwich"}}
{"id":"mt-sandwich","label":"sandwich","field":"meal_type","aliases":["steak sandwich","sub","toastie","panini"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-hot-dog"}}
{"id":"mt-hot-dog","label":"hot dog","field":"meal_type","aliases":["hotdog","frankfurter"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-fish-and-chips"}}
{"id":"mt-fish-and-chips","label":"fish and chips","field":"meal_type","aliases":["fish n chips","fish & chips","chippy"],"weight":11,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-roast"}}
{"id":"mt-roast","label":"roast","field":"meal_type","aliases":["roast dinner","sunday roast","roast chicken"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-schnitzel"}}
{"id":"mt-schnitzel","label":"schnitzel","field":"meal_type","aliases":["chicken schnitzel","pork schnitzel","crumbed"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-noodle-soup"}}
{"id":"mt-noodle-soup","label":"noodle soup","field":"meal_type","aliases":["ramen","pho","laksa"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-fried-rice"}}
{"id":"mt-fried-rice","label":"fried rice","field":"meal_type","aliases":["egg fried rice","special fried rice"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-bibimbap"}}
{"id":"mt-bibimbap","label":"bibimbap","field":"meal_type","aliases":["korean rice bowl","bibimbap bowl"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-bulgogi"}}
{"id":"mt-bulgogi","label":"bulgogi","field":"meal_type","aliases":["korean bbq","bulgogi beef","bulgogi chicken"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-moussaka"}}
{"id":"mt-moussaka","label":"moussaka","field":"meal_type","aliases":["greek moussaka","lentil moussaka"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-ratatouille"}}
{"id":"mt-ratatouille","label":"ratatouille","field":"meal_type","aliases":["french ratatouille","provencal vegetables"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-borek"}}
{"id":"mt-borek","label":"borek","field":"meal_type","aliases":["turkish borek","cheese borek","filo borek"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-pide"}}
{"id":"mt-pide","label":"pide","field":"meal_type","aliases":["turkish pizza","turkish flatbread"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-crumble"}}
{"id":"mt-crumble","label":"crumble","field":"meal_type","aliases":["savoury crumble","vegetable crumble"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-traybake"}}
{"id":"mt-traybake","label":"traybake","field":"meal_type","aliases":["sheet pan dinner","one tray","oven traybake"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-rice-bowl"}}
{"id":"mt-rice-bowl","label":"rice bowl","field":"meal_type","aliases":["bowl","grain bowl","loaded rice bowl"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"mt-spring-roll"}}
{"id":"mt-spring-roll","label":"spring roll","field":"meal_type","aliases":["spring rolls","vietnamese spring roll","crispy roll"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-italian"}}
{"id":"cui-italian","label":"italian","field":"cuisine","aliases":["italia","mediterranean italian","tuscan"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-mexican"}}
{"id":"cui-mexican","label":"mexican","field":"cuisine","aliases":["tex mex","tex-mex","latino"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-indian"}}
{"id":"cui-indian","label":"indian","field":"cuisine","aliases":["south asian","desi","indo"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-thai"}}
{"id":"cui-thai","label":"thai","field":"cuisine","aliases":["thailand","southeast asian"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-chinese"}}
{"id":"cui-chinese","label":"chinese","field":"cuisine","aliases":["cantonese","szechuan","sichuan"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-japanese"}}
{"id":"cui-japanese","label":"japanese","field":"cuisine","aliases":["japan","nippon"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-korean"}}
{"id":"cui-korean","label":"korean","field":"cuisine","aliases":["k-food","south korean"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-vietnamese"}}
{"id":"cui-vietnamese","label":"vietnamese","field":"cuisine","aliases":["vietnam"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-indonesian"}}
{"id":"cui-indonesian","label":"indonesian","field":"cuisine","aliases":["indo chinese"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-greek"}}
{"id":"cui-greek","label":"greek","field":"cuisine","aliases":["grecian","hellenic"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-turkish"}}
{"id":"cui-turkish","label":"turkish","field":"cuisine","aliases":["ottoman","anatolian"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-lebanese"}}
{"id":"cui-lebanese","label":"lebanese","field":"cuisine","aliases":["levantine","middle eastern"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-moroccan"}}
{"id":"cui-moroccan","label":"moroccan","field":"cuisine","aliases":["north african","maghreb"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-british"}}
{"id":"cui-british","label":"british","field":"cuisine","aliases":["english","traditional british","classic british"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-american"}}
{"id":"cui-american","label":"american","field":"cuisine","aliases":["usa","us style"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-french"}}
{"id":"cui-french","label":"french","field":"cuisine","aliases":["francais","bistro","provencal"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-spanish"}}
{"id":"cui-spanish","label":"spanish","field":"cuisine","aliases":["espanol","iberian"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-portuguese"}}
{"id":"cui-portuguese","label":"portuguese","field":"cuisine","aliases":["peri peri","nandos style"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-german"}}
{"id":"cui-german","label":"german","field":"cuisine","aliases":["deutsch","bavarian"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-mediterranean"}}
{"id":"cui-mediterranean","label":"mediterranean","field":"cuisine","aliases":["med","coastal"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-caribbean"}}
{"id":"cui-caribbean","label":"caribbean","field":"cuisine","aliases":["west indian","jamaican","trinidadian"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-african"}}
{"id":"cui-african","label":"african","field":"cuisine","aliases":["west african","east african"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-georgian"}}
{"id":"cui-georgian","label":"georgian","field":"cuisine","aliases":["caucasian","eastern european"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-cajun"}}
{"id":"cui-cajun","label":"cajun","field":"cuisine","aliases":["creole","louisiana","southern american"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-nordic"}}
{"id":"cui-nordic","label":"nordic","field":"cuisine","aliases":["scandinavian","swedish","danish","norwegian"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"cui-sri-lankan"}}
{"id":"cui-sri-lankan","label":"sri lankan","field":"cuisine","aliases":["ceylon","sinhalese"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-vegetarian"}}
{"id":"diet-vegetarian","label":"vegetarian","field":"dietary","aliases":["veggie","meat free","meatless","no meat"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-vegan"}}
{"id":"diet-vegan","label":"vegan","field":"dietary","aliases":["plant based","plant-based","no animal products"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-pescatarian"}}
{"id":"diet-pescatarian","label":"pescatarian","field":"dietary","aliases":["fish only","no meat but fish"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-gluten-free"}}
{"id":"diet-gluten-free","label":"gluten free","field":"dietary","aliases":["gf","coeliac","celiac","no gluten"],"weight":10,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-low-carb"}}
{"id":"diet-low-carb","label":"low carb","field":"dietary","aliases":["keto","low carbohydrate","carb free","ketogenic"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-low-calorie"}}
{"id":"diet-low-calorie","label":"low calorie","field":"dietary","aliases":["light","calorie smart","under 600 calories","diet","healthy"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-high-protein"}}
{"id":"diet-high-protein","label":"high protein","field":"dietary","aliases":["protein rich","protein packed","high in protein"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-dairy-free"}}
{"id":"diet-dairy-free","label":"dairy free","field":"dietary","aliases":["no dairy","lactose free","without dairy"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-nut-free"}}
{"id":"diet-nut-free","label":"nut free","field":"dietary","aliases":["no nuts","peanut free","without nuts"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-family-friendly"}}
{"id":"diet-family-friendly","label":"family friendly","field":"dietary","aliases":["kid friendly","family","for kids","child friendly"],"weight":9,"locale":"en_GB","market":"GB"}
{"index":{"_id":"diet-climate-conscious"}}
{"id":"diet-climate-conscious","label":"climate conscious","field":"dietary","aliases":["sustainable","eco friendly","low carbon"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-grilled"}}
{"id":"method-grilled","label":"grilled","field":"cooking_method","aliases":["grilling","barbecued","bbq","chargrilled","char grilled"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-baked"}}
{"id":"method-baked","label":"baked","field":"cooking_method","aliases":["oven baked","roasted","oven roasted"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-fried"}}
{"id":"method-fried","label":"fried","field":"cooking_method","aliases":["pan fried","pan-fried","deep fried","shallow fried","crispy"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-slow-cooked"}}
{"id":"method-slow-cooked","label":"slow cooked","field":"cooking_method","aliases":["slow cooker","crockpot","braised","low and slow"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-steamed"}}
{"id":"method-steamed","label":"steamed","field":"cooking_method","aliases":["steaming"],"weight":7,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-air-fryer"}}
{"id":"method-air-fryer","label":"air fryer","field":"cooking_method","aliases":["air fried","airfryer","air fry"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-one-pot"}}
{"id":"method-one-pot","label":"one pot","field":"cooking_method","aliases":["one pan","sheet pan","traybake","one tray"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-quick"}}
{"id":"method-quick","label":"quick","field":"cooking_method","aliases":["15 minute","20 minute","speedy","fast","easy","rapid","prepped in 10"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-under-30"}}
{"id":"method-under-30","label":"under 30 minutes","field":"cooking_method","aliases":["30 min","30 minutes","quick meal","30 minute meal"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-date-night"}}
{"id":"method-date-night","label":"date night","field":"cooking_method","aliases":["romantic dinner","special occasion","posh"],"weight":8,"locale":"en_GB","market":"GB"}
{"index":{"_id":"method-street-food"}}
{"id":"method-street-food","label":"street food","field":"cooking_method","aliases":["hawker","food truck","market food"],"weight":8,"locale":"en_GB","market":"GB"}
NDJSON

bulk_index "concepts_en_gb" "${TMPDIR}/concepts_en_gb.ndjson"

# ============================================================================
# 4b. CONCEPTS — US market (en_US / US)
#     Sources: hellofresh.com/recipes, popular, american-recipes
#     US-specific proteins, ingredients, meal types, cuisines, dietary labels
# ============================================================================
echo "==> Indexing US concepts..."

cat > "${TMPDIR}/concepts_en_us.ndjson" << 'NDJSON'
{"index":{"_id":"us-cat-chicken"}}
{"id":"us-cat-chicken","label":"chicken","field":"category","aliases":["poultry"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chicken-breast"}}
{"id":"us-cat-chicken-breast","label":"chicken breast","field":"category","aliases":["chicken cutlet","boneless skinless chicken breast"],"weight":12,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chicken-thigh"}}
{"id":"us-cat-chicken-thigh","label":"chicken thigh","field":"category","aliases":["boneless chicken thigh","dark meat"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chicken-tender"}}
{"id":"us-cat-chicken-tender","label":"chicken tenders","field":"category","aliases":["chicken tender","chicken strips","chicken fingers"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chicken-sausage"}}
{"id":"us-cat-chicken-sausage","label":"chicken sausage","field":"category","aliases":["poultry sausage"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ground-chicken"}}
{"id":"us-cat-ground-chicken","label":"ground chicken","field":"category","aliases":["minced chicken"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-beef"}}
{"id":"us-cat-beef","label":"beef","field":"category","aliases":["cow","steer"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ground-beef"}}
{"id":"us-cat-ground-beef","label":"ground beef","field":"category","aliases":["minced beef","hamburger meat","ground chuck"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-sirloin"}}
{"id":"us-cat-sirloin","label":"sirloin steak","field":"category","aliases":["sirloin","top sirloin","sirloin tips"],"weight":12,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ny-strip"}}
{"id":"us-cat-ny-strip","label":"new york strip","field":"category","aliases":["ny strip","strip steak","new york strip steak"],"weight":12,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ribeye"}}
{"id":"us-cat-ribeye","label":"rib eye","field":"category","aliases":["ribeye","rib eye steak","ribeye steak"],"weight":12,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-beef-tenderloin"}}
{"id":"us-cat-beef-tenderloin","label":"beef tenderloin","field":"category","aliases":["filet mignon","tenderloin steak"],"weight":12,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-bavette"}}
{"id":"us-cat-bavette","label":"bavette steak","field":"category","aliases":["bavette","flank steak","flap steak"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-beef-meatball"}}
{"id":"us-cat-beef-meatball","label":"beef meatballs","field":"category","aliases":["meatballs","meatball"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-meatloaf"}}
{"id":"us-cat-meatloaf","label":"meatloaf","field":"category","aliases":["meat loaf","meatloaves"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pork"}}
{"id":"us-cat-pork","label":"pork","field":"category","aliases":["pig"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pork-chop"}}
{"id":"us-cat-pork-chop","label":"pork chop","field":"category","aliases":["pork chops","bone in pork chop","center cut pork chop"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pork-tenderloin"}}
{"id":"us-cat-pork-tenderloin","label":"pork tenderloin","field":"category","aliases":["pork loin","pork fillet"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ground-pork"}}
{"id":"us-cat-ground-pork","label":"ground pork","field":"category","aliases":["minced pork","pork mince"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pulled-pork"}}
{"id":"us-cat-pulled-pork","label":"pulled pork","field":"category","aliases":["carnitas","slow cooked pork","smoked pork"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-bacon"}}
{"id":"us-cat-bacon","label":"bacon","field":"category","aliases":["bacon strips","crispy bacon","applewood bacon","smoked bacon"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pancetta"}}
{"id":"us-cat-pancetta","label":"pancetta","field":"category","aliases":["italian bacon"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-prosciutto"}}
{"id":"us-cat-prosciutto","label":"prosciutto","field":"category","aliases":["prosciutto di parma","italian ham"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-sausage"}}
{"id":"us-cat-sausage","label":"sausage","field":"category","aliases":["pork sausage","italian sausage","breakfast sausage","sausage links"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-turkey"}}
{"id":"us-cat-turkey","label":"turkey","field":"category","aliases":["ground turkey","turkey cutlet","turkey breast"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ground-turkey"}}
{"id":"us-cat-ground-turkey","label":"ground turkey","field":"category","aliases":["minced turkey","turkey mince"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-lamb"}}
{"id":"us-cat-lamb","label":"lamb","field":"category","aliases":["lamb chops","rack of lamb"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-salmon"}}
{"id":"us-cat-salmon","label":"salmon","field":"category","aliases":["salmon fillet","atlantic salmon","wild salmon","salmon filet"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cod"}}
{"id":"us-cat-cod","label":"cod","field":"category","aliases":["cod fillet","atlantic cod","cod filet"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-shrimp"}}
{"id":"us-cat-shrimp","label":"shrimp","field":"category","aliases":["prawns","jumbo shrimp","gulf shrimp","shrimp cocktail"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-scallops"}}
{"id":"us-cat-scallops","label":"scallops","field":"category","aliases":["sea scallops","bay scallops","pan seared scallops"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tilapia"}}
{"id":"us-cat-tilapia","label":"tilapia","field":"category","aliases":["tilapia fillet","tilapia filet","white fish"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-trout"}}
{"id":"us-cat-trout","label":"trout","field":"category","aliases":["rainbow trout","trout fillet"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tuna"}}
{"id":"us-cat-tuna","label":"tuna","field":"category","aliases":["tuna steak","ahi tuna","canned tuna","yellowfin"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-lobster"}}
{"id":"us-cat-lobster","label":"lobster","field":"category","aliases":["lobster tail","lobster ravioli"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tofu"}}
{"id":"us-cat-tofu","label":"tofu","field":"category","aliases":["bean curd","firm tofu","extra firm tofu"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-egg"}}
{"id":"us-cat-egg","label":"egg","field":"ingredient","aliases":["eggs","fried egg","poached egg","scrambled egg"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pasta"}}
{"id":"us-cat-pasta","label":"pasta","field":"category","aliases":["italian pasta","dried pasta"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-spaghetti"}}
{"id":"us-cat-spaghetti","label":"spaghetti","field":"category","aliases":["spaghetti pasta","thin spaghetti"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-penne"}}
{"id":"us-cat-penne","label":"penne","field":"category","aliases":["penne pasta","penne rigate","mostaccioli"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-rigatoni"}}
{"id":"us-cat-rigatoni","label":"rigatoni","field":"category","aliases":["rigatoni pasta"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-orzo"}}
{"id":"us-cat-orzo","label":"orzo","field":"category","aliases":["orzo pasta","risoni"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cavatappi"}}
{"id":"us-cat-cavatappi","label":"cavatappi","field":"category","aliases":["cavatappi pasta","corkscrew pasta","cellentani"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tagliatelle"}}
{"id":"us-cat-tagliatelle","label":"tagliatelle","field":"category","aliases":["tagliatelle pasta","fettuccine","fettuccini"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-gemelli"}}
{"id":"us-cat-gemelli","label":"gemelli","field":"category","aliases":["gemelli pasta","twisted pasta"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tortelloni"}}
{"id":"us-cat-tortelloni","label":"tortelloni","field":"category","aliases":["tortellini","cheese tortelloni","stuffed pasta"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ravioli"}}
{"id":"us-cat-ravioli","label":"ravioli","field":"category","aliases":["lobster ravioli","cheese ravioli","filled pasta"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-agnolotti"}}
{"id":"us-cat-agnolotti","label":"agnolotti","field":"category","aliases":["butternut squash agnolotti","stuffed pasta"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-gnocchi"}}
{"id":"us-cat-gnocchi","label":"gnocchi","field":"category","aliases":["potato gnocchi"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-macaroni"}}
{"id":"us-cat-macaroni","label":"macaroni","field":"category","aliases":["mac","elbow macaroni","mac and cheese"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-rice"}}
{"id":"us-cat-rice","label":"rice","field":"category","aliases":["white rice","long grain rice"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-jasmine-rice"}}
{"id":"us-cat-jasmine-rice","label":"jasmine rice","field":"category","aliases":["jasmine","thai rice","fragrant rice"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-basmati"}}
{"id":"us-cat-basmati","label":"basmati rice","field":"category","aliases":["basmati","aromatic rice"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-arborio"}}
{"id":"us-cat-arborio","label":"arborio rice","field":"category","aliases":["arborio","risotto rice"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-quinoa"}}
{"id":"us-cat-quinoa","label":"quinoa","field":"category","aliases":["white quinoa","red quinoa","quinoa bowl"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-couscous"}}
{"id":"us-cat-couscous","label":"couscous","field":"category","aliases":["pearled couscous","israeli couscous"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-noodles"}}
{"id":"us-cat-noodles","label":"noodles","field":"category","aliases":["egg noodles","ramen noodles","lo mein noodles","rice noodles"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-polenta"}}
{"id":"us-cat-polenta","label":"polenta","field":"category","aliases":["creamy polenta","cornmeal","grits"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-potato"}}
{"id":"us-cat-potato","label":"potato","field":"ingredient","aliases":["potatoes","russet","yukon gold","fingerling","mashed potatoes","baked potato"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-sweet-potato"}}
{"id":"us-cat-sweet-potato","label":"sweet potato","field":"ingredient","aliases":["sweet potatoes","yam","sweet potato fries"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-fries"}}
{"id":"us-cat-fries","label":"fries","field":"ingredient","aliases":["french fries","steak fries","seasoned fries","wedges","potato wedges","tater tots"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-broccoli"}}
{"id":"us-cat-broccoli","label":"broccoli","field":"ingredient","aliases":["broccoli florets","broccoli crowns","broccolini"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cauliflower"}}
{"id":"us-cat-cauliflower","label":"cauliflower","field":"ingredient","aliases":["cauliflower florets","cauliflower rice","riced cauliflower"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-brussels-sprouts"}}
{"id":"us-cat-brussels-sprouts","label":"brussels sprouts","field":"ingredient","aliases":["brussel sprouts","sprouts","roasted brussels sprouts"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-kale"}}
{"id":"us-cat-kale","label":"kale","field":"ingredient","aliases":["curly kale","baby kale","tuscan kale","lacinato kale","creamed kale"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-spinach"}}
{"id":"us-cat-spinach","label":"spinach","field":"ingredient","aliases":["baby spinach","spinach leaves","creamed spinach"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-asparagus"}}
{"id":"us-cat-asparagus","label":"asparagus","field":"ingredient","aliases":["asparagus spears","asparagus tips"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-green-beans"}}
{"id":"us-cat-green-beans","label":"green beans","field":"ingredient","aliases":["string beans","snap beans","haricot vert","green beans almandine"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-zucchini"}}
{"id":"us-cat-zucchini","label":"zucchini","field":"ingredient","aliases":["courgette","summer squash","yellow squash"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-corn"}}
{"id":"us-cat-corn","label":"corn","field":"ingredient","aliases":["sweet corn","corn on the cob","corn kernels","baby corn","charred corn"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-bell-pepper"}}
{"id":"us-cat-bell-pepper","label":"bell pepper","field":"ingredient","aliases":["pepper","red bell pepper","green bell pepper","yellow bell pepper","capsicum"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-poblano"}}
{"id":"us-cat-poblano","label":"poblano pepper","field":"ingredient","aliases":["poblano","ancho pepper","stuffed pepper"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-jalapeno"}}
{"id":"us-cat-jalapeno","label":"jalapeno","field":"ingredient","aliases":["jalapeño","hot pepper","pickled jalapeno"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-onion"}}
{"id":"us-cat-onion","label":"onion","field":"ingredient","aliases":["onions","red onion","yellow onion","white onion","sweet onion","vidalia"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-scallion"}}
{"id":"us-cat-scallion","label":"scallion","field":"ingredient","aliases":["scallions","green onion","green onions","spring onion"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-shallot"}}
{"id":"us-cat-shallot","label":"shallot","field":"ingredient","aliases":["shallots","echalion"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-garlic"}}
{"id":"us-cat-garlic","label":"garlic","field":"ingredient","aliases":["garlic clove","garlic cloves","minced garlic","roasted garlic"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tomato"}}
{"id":"us-cat-tomato","label":"tomato","field":"ingredient","aliases":["tomatoes","cherry tomatoes","heirloom tomato","diced tomatoes","sun dried tomatoes","grape tomatoes"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-mushroom"}}
{"id":"us-cat-mushroom","label":"mushroom","field":"ingredient","aliases":["mushrooms","cremini","baby bella","portobello","shiitake","button mushroom"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-butternut-squash"}}
{"id":"us-cat-butternut-squash","label":"butternut squash","field":"ingredient","aliases":["squash","winter squash","roasted squash"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-carrot"}}
{"id":"us-cat-carrot","label":"carrot","field":"ingredient","aliases":["carrots","baby carrots","shredded carrots"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-parsnip"}}
{"id":"us-cat-parsnip","label":"parsnip","field":"ingredient","aliases":["parsnips","parsnip wedges"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cabbage"}}
{"id":"us-cat-cabbage","label":"cabbage","field":"ingredient","aliases":["napa cabbage","red cabbage","green cabbage","coleslaw"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-bok-choy"}}
{"id":"us-cat-bok-choy","label":"bok choy","field":"ingredient","aliases":["baby bok choy","chinese cabbage","pak choi"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-edamame"}}
{"id":"us-cat-edamame","label":"edamame","field":"ingredient","aliases":["soybeans","shelled edamame"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-arugula"}}
{"id":"us-cat-arugula","label":"arugula","field":"ingredient","aliases":["rocket","baby arugula","arugula salad"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-lettuce"}}
{"id":"us-cat-lettuce","label":"lettuce","field":"ingredient","aliases":["romaine","iceberg","mixed greens","spring mix","butter lettuce"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cucumber"}}
{"id":"us-cat-cucumber","label":"cucumber","field":"ingredient","aliases":["persian cucumber","english cucumber","pickled cucumber"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-avocado"}}
{"id":"us-cat-avocado","label":"avocado","field":"ingredient","aliases":["avo","guacamole","avocado crema"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pineapple"}}
{"id":"us-cat-pineapple","label":"pineapple","field":"ingredient","aliases":["pineapple chunks","pineapple relish","grilled pineapple"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cranberry"}}
{"id":"us-cat-cranberry","label":"cranberry","field":"ingredient","aliases":["cranberries","dried cranberries","cranberry sauce"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-fig"}}
{"id":"us-cat-fig","label":"fig","field":"ingredient","aliases":["figs","fig jam","jammy fig"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-black-beans"}}
{"id":"us-cat-black-beans","label":"black beans","field":"category","aliases":["canned black beans","refried beans"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chickpeas"}}
{"id":"us-cat-chickpeas","label":"chickpeas","field":"category","aliases":["garbanzo beans","canned chickpeas"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-lentils"}}
{"id":"us-cat-lentils","label":"lentils","field":"category","aliases":["red lentils","green lentils","brown lentils"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-parmesan"}}
{"id":"us-cat-parmesan","label":"parmesan","field":"ingredient","aliases":["parmigiano reggiano","parm","grated parmesan"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-mozzarella"}}
{"id":"us-cat-mozzarella","label":"mozzarella","field":"ingredient","aliases":["fresh mozzarella","shredded mozzarella","mozzarella cheese"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cheddar"}}
{"id":"us-cat-cheddar","label":"cheddar","field":"ingredient","aliases":["sharp cheddar","mild cheddar","cheddar cheese","shredded cheddar"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-monterey-jack"}}
{"id":"us-cat-monterey-jack","label":"monterey jack","field":"ingredient","aliases":["pepper jack","jack cheese","colby jack"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-gouda"}}
{"id":"us-cat-gouda","label":"gouda","field":"ingredient","aliases":["smoked gouda","aged gouda","gouda cheese"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-brie"}}
{"id":"us-cat-brie","label":"brie","field":"ingredient","aliases":["brie cheese","double cream brie"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-feta"}}
{"id":"us-cat-feta","label":"feta","field":"ingredient","aliases":["feta cheese","crumbled feta"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cream-cheese"}}
{"id":"us-cat-cream-cheese","label":"cream cheese","field":"ingredient","aliases":["philly","cream cheese spread"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-sour-cream"}}
{"id":"us-cat-sour-cream","label":"sour cream","field":"ingredient","aliases":["soured cream","dollop"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-creme-fraiche"}}
{"id":"us-cat-creme-fraiche","label":"creme fraiche","field":"ingredient","aliases":["crème fraîche"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-butter"}}
{"id":"us-cat-butter","label":"butter","field":"ingredient","aliases":["unsalted butter","salted butter","compound butter"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-olive-oil"}}
{"id":"us-cat-olive-oil","label":"olive oil","field":"ingredient","aliases":["extra virgin olive oil","evoo"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tortilla"}}
{"id":"us-cat-tortilla","label":"tortilla","field":"ingredient","aliases":["flour tortilla","corn tortilla","tortillas","soft tortilla"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-burger-bun"}}
{"id":"us-cat-burger-bun","label":"burger bun","field":"ingredient","aliases":["brioche bun","sesame bun","potato bun"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-flatbread"}}
{"id":"us-cat-flatbread","label":"flatbread","field":"ingredient","aliases":["naan","pita","lavash"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-panko"}}
{"id":"us-cat-panko","label":"panko breadcrumbs","field":"ingredient","aliases":["panko","breadcrumbs","crispy coating"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-soy-sauce"}}
{"id":"us-cat-soy-sauce","label":"soy sauce","field":"ingredient","aliases":["soya sauce","shoyu","tamari","low sodium soy sauce"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ginger"}}
{"id":"us-cat-ginger","label":"ginger","field":"ingredient","aliases":["fresh ginger","ginger root","ground ginger","ginger paste"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chili"}}
{"id":"us-cat-chili","label":"chili","field":"ingredient","aliases":["chilli","red chili","green chili","chili flakes","red pepper flakes"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cumin"}}
{"id":"us-cat-cumin","label":"cumin","field":"ingredient","aliases":["ground cumin","cumin seeds"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-paprika"}}
{"id":"us-cat-paprika","label":"paprika","field":"ingredient","aliases":["smoked paprika","sweet paprika","hot paprika"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-cilantro"}}
{"id":"us-cat-cilantro","label":"cilantro","field":"ingredient","aliases":["fresh cilantro","coriander"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-old-bay"}}
{"id":"us-cat-old-bay","label":"old bay","field":"ingredient","aliases":["old bay seasoning","seafood seasoning"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-italian-seasoning"}}
{"id":"us-cat-italian-seasoning","label":"italian seasoning","field":"ingredient","aliases":["tuscan herbs","italian herbs","herbs de provence"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-chipotle"}}
{"id":"us-cat-chipotle","label":"chipotle","field":"ingredient","aliases":["chipotle pepper","chipotle seasoning","chipotle in adobo","smoked jalapeno"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-ranch"}}
{"id":"us-cat-ranch","label":"ranch","field":"ingredient","aliases":["ranch dressing","ranch seasoning","ranch sauce","buttermilk ranch"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-bbq-sauce"}}
{"id":"us-cat-bbq-sauce","label":"bbq sauce","field":"ingredient","aliases":["barbecue sauce","bbq","smoky bbq","honey bbq"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-honey-mustard"}}
{"id":"us-cat-honey-mustard","label":"honey mustard","field":"ingredient","aliases":["honey mustard sauce","honey mustard dressing"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-hot-sauce"}}
{"id":"us-cat-hot-sauce","label":"hot sauce","field":"ingredient","aliases":["buffalo sauce","franks","sriracha","tabasco"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pesto"}}
{"id":"us-cat-pesto","label":"pesto","field":"ingredient","aliases":["basil pesto","green pesto"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-marinara"}}
{"id":"us-cat-marinara","label":"marinara","field":"ingredient","aliases":["marinara sauce","tomato sauce","pomodoro"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-alfredo-sauce"}}
{"id":"us-cat-alfredo-sauce","label":"alfredo sauce","field":"ingredient","aliases":["alfredo","creamy alfredo","white sauce"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-teriyaki"}}
{"id":"us-cat-teriyaki","label":"teriyaki","field":"ingredient","aliases":["teriyaki sauce","teriyaki glaze"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-hoisin"}}
{"id":"us-cat-hoisin","label":"hoisin","field":"ingredient","aliases":["hoisin sauce"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pico"}}
{"id":"us-cat-pico","label":"pico de gallo","field":"ingredient","aliases":["pico","fresh salsa","salsa fresca"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-salsa"}}
{"id":"us-cat-salsa","label":"salsa","field":"ingredient","aliases":["tomato salsa","verde salsa","salsa verde","salsa roja"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-lime-crema"}}
{"id":"us-cat-lime-crema","label":"lime crema","field":"ingredient","aliases":["crema","sriracha crema","avocado crema","chili lime crema"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-hummus"}}
{"id":"us-cat-hummus","label":"hummus","field":"ingredient","aliases":["humous","houmous","chickpea dip"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tzatziki"}}
{"id":"us-cat-tzatziki","label":"tzatziki","field":"ingredient","aliases":["tzatziki sauce","cucumber yogurt sauce"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-tahini"}}
{"id":"us-cat-tahini","label":"tahini","field":"ingredient","aliases":["tahini paste","sesame paste"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-truffle"}}
{"id":"us-cat-truffle","label":"truffle","field":"ingredient","aliases":["truffle oil","truffle sauce","truffled","truffle butter"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-maple"}}
{"id":"us-cat-maple","label":"maple","field":"ingredient","aliases":["maple syrup","maple glaze","maple dijon","maple balsamic"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-bourbon"}}
{"id":"us-cat-bourbon","label":"bourbon","field":"ingredient","aliases":["bourbon glaze","brown sugar bourbon"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-balsamic"}}
{"id":"us-cat-balsamic","label":"balsamic","field":"ingredient","aliases":["balsamic vinegar","balsamic glaze","balsamic reduction"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-peppercorn-sauce"}}
{"id":"us-cat-peppercorn-sauce","label":"peppercorn sauce","field":"ingredient","aliases":["peppercorn","cracked peppercorn","au poivre"],"weight":7,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pecans"}}
{"id":"us-cat-pecans","label":"pecans","field":"ingredient","aliases":["candied pecans","pecan","toasted pecans"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-walnuts"}}
{"id":"us-cat-walnuts","label":"walnuts","field":"ingredient","aliases":["walnut","chopped walnuts","toasted walnuts"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-almonds"}}
{"id":"us-cat-almonds","label":"almonds","field":"ingredient","aliases":["sliced almonds","slivered almonds","almond"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cat-pistachios"}}
{"id":"us-cat-pistachios","label":"pistachios","field":"ingredient","aliases":["pistachio","shelled pistachios"],"weight":6,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-burger"}}
{"id":"us-mt-burger","label":"burger","field":"meal_type","aliases":["hamburger","cheeseburger","smash burger","smashed burger","turkey burger","veggie burger"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-stir-fry"}}
{"id":"us-mt-stir-fry","label":"stir fry","field":"meal_type","aliases":["stirfry","wok","stir-fry"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-tacos"}}
{"id":"us-mt-tacos","label":"tacos","field":"meal_type","aliases":["taco","fish tacos","shrimp tacos","crispy tacos","street tacos"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-burrito"}}
{"id":"us-mt-burrito","label":"burrito","field":"meal_type","aliases":["burrito bowl","bean burrito","chicken burrito","breakfast burrito"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-fajitas"}}
{"id":"us-mt-fajitas","label":"fajitas","field":"meal_type","aliases":["fajita","chicken fajitas","steak fajitas"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-enchiladas"}}
{"id":"us-mt-enchiladas","label":"enchiladas","field":"meal_type","aliases":["enchilada","chicken enchiladas","cheese enchiladas"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-quesadilla"}}
{"id":"us-mt-quesadilla","label":"quesadilla","field":"meal_type","aliases":["quesadillas","chicken quesadilla","cheese quesadilla"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-tostada"}}
{"id":"us-mt-tostada","label":"tostada","field":"meal_type","aliases":["tostadas","crispy tostada"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-nachos"}}
{"id":"us-mt-nachos","label":"nachos","field":"meal_type","aliases":["loaded nachos","cheesy nachos"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-chili"}}
{"id":"us-mt-chili","label":"chili","field":"meal_type","aliases":["chilli","chili con carne","white chicken chili","turkey chili","bean chili"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-pasta"}}
{"id":"us-mt-pasta","label":"pasta","field":"meal_type","aliases":["pasta dish","baked pasta","pasta bake"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-bolognese"}}
{"id":"us-mt-bolognese","label":"bolognese","field":"meal_type","aliases":["meat sauce","ragu","spaghetti bolognese"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-mac-and-cheese"}}
{"id":"us-mt-mac-and-cheese","label":"mac and cheese","field":"meal_type","aliases":["mac n cheese","macaroni and cheese","baked mac and cheese"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-carbonara"}}
{"id":"us-mt-carbonara","label":"carbonara","field":"meal_type","aliases":["spaghetti carbonara","pasta carbonara"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-alfredo"}}
{"id":"us-mt-alfredo","label":"alfredo","field":"meal_type","aliases":["fettuccine alfredo","chicken alfredo","shrimp alfredo"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-chicken-parm"}}
{"id":"us-mt-chicken-parm","label":"chicken parm","field":"meal_type","aliases":["chicken parmesan","chicken parmigiana","chicken parmagiana"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-lasagna"}}
{"id":"us-mt-lasagna","label":"lasagna","field":"meal_type","aliases":["lasagne","baked lasagna","meat lasagna"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-risotto"}}
{"id":"us-mt-risotto","label":"risotto","field":"meal_type","aliases":["shrimp risotto","mushroom risotto"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-pizza"}}
{"id":"us-mt-pizza","label":"pizza","field":"meal_type","aliases":["flatbread pizza","homemade pizza","personal pizza"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-sandwich"}}
{"id":"us-mt-sandwich","label":"sandwich","field":"meal_type","aliases":["grilled cheese","sando","sub","panini","club sandwich","french dip"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-meatloaf"}}
{"id":"us-mt-meatloaf","label":"meatloaf","field":"meal_type","aliases":["meat loaf","turkey meatloaf","meatloaf balsamico"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-meatball"}}
{"id":"us-mt-meatball","label":"meatballs","field":"meal_type","aliases":["meatball","spaghetti and meatballs","swedish meatballs"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-casserole"}}
{"id":"us-mt-casserole","label":"casserole","field":"meal_type","aliases":["one pot","hotdish","bake"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-skillet"}}
{"id":"us-mt-skillet","label":"skillet","field":"meal_type","aliases":["one pan","skillet dinner","sheet pan","one skillet"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-bowl"}}
{"id":"us-mt-bowl","label":"bowl","field":"meal_type","aliases":["grain bowl","rice bowl","buddha bowl","power bowl","poke bowl"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-bibimbap"}}
{"id":"us-mt-bibimbap","label":"bibimbap","field":"meal_type","aliases":["korean bibimbap","korean rice bowl"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-soup"}}
{"id":"us-mt-soup","label":"soup","field":"meal_type","aliases":["chowder","bisque","stew","broth"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-salad"}}
{"id":"us-mt-salad","label":"salad","field":"meal_type","aliases":["side salad","chicken salad","cobb salad","caesar salad","harvest salad"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-curry"}}
{"id":"us-mt-curry","label":"curry","field":"meal_type","aliases":["indian curry","thai curry","coconut curry","butter chicken"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-steak-and-potatoes"}}
{"id":"us-mt-steak-and-potatoes","label":"steak and potatoes","field":"meal_type","aliases":["steak dinner","steak night","surf and turf"],"weight":11,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-fried-rice"}}
{"id":"us-mt-fried-rice","label":"fried rice","field":"meal_type","aliases":["egg fried rice","chicken fried rice"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-noodle-soup"}}
{"id":"us-mt-noodle-soup","label":"noodle soup","field":"meal_type","aliases":["ramen","pho"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-stuffed-pepper"}}
{"id":"us-mt-stuffed-pepper","label":"stuffed peppers","field":"meal_type","aliases":["stuffed pepper","stuffed bell pepper"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-mt-wrap"}}
{"id":"us-mt-wrap","label":"wrap","field":"meal_type","aliases":["chicken wrap","lettuce wrap","tortilla wrap"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-american"}}
{"id":"us-cui-american","label":"american","field":"cuisine","aliases":["usa","classic american","all american","comfort food"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-italian"}}
{"id":"us-cui-italian","label":"italian","field":"cuisine","aliases":["tuscan","venetian","sicilian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-mexican"}}
{"id":"us-cui-mexican","label":"mexican","field":"cuisine","aliases":["tex mex","tex-mex","southwest","southwestern"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-indian"}}
{"id":"us-cui-indian","label":"indian","field":"cuisine","aliases":["south asian","desi"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-thai"}}
{"id":"us-cui-thai","label":"thai","field":"cuisine","aliases":["thailand","southeast asian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-chinese"}}
{"id":"us-cui-chinese","label":"chinese","field":"cuisine","aliases":["cantonese","szechuan","sichuan","mandarin"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-japanese"}}
{"id":"us-cui-japanese","label":"japanese","field":"cuisine","aliases":["japan"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-korean"}}
{"id":"us-cui-korean","label":"korean","field":"cuisine","aliases":["k-food","south korean","korean bbq"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-vietnamese"}}
{"id":"us-cui-vietnamese","label":"vietnamese","field":"cuisine","aliases":["vietnam"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-mediterranean"}}
{"id":"us-cui-mediterranean","label":"mediterranean","field":"cuisine","aliases":["med","coastal","mediterranean diet"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-greek"}}
{"id":"us-cui-greek","label":"greek","field":"cuisine","aliases":["grecian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-french"}}
{"id":"us-cui-french","label":"french","field":"cuisine","aliases":["provencal","bistro","parisian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-spanish"}}
{"id":"us-cui-spanish","label":"spanish","field":"cuisine","aliases":["iberian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-latin"}}
{"id":"us-cui-latin","label":"latin american","field":"cuisine","aliases":["latino","latin","south american","cuban","peruvian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-cajun"}}
{"id":"us-cui-cajun","label":"cajun","field":"cuisine","aliases":["creole","louisiana","southern","new orleans"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-hawaiian"}}
{"id":"us-cui-hawaiian","label":"hawaiian","field":"cuisine","aliases":["hawaii","island style","aloha","poke"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-african"}}
{"id":"us-cui-african","label":"african","field":"cuisine","aliases":["west african","ethiopian","moroccan"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-cui-middle-eastern"}}
{"id":"us-cui-middle-eastern","label":"middle eastern","field":"cuisine","aliases":["lebanese","turkish","persian"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-vegetarian"}}
{"id":"us-diet-vegetarian","label":"vegetarian","field":"dietary","aliases":["veggie","meat free","meatless","no meat"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-vegan"}}
{"id":"us-diet-vegan","label":"vegan","field":"dietary","aliases":["plant based","plant-based","no animal products"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-pescatarian"}}
{"id":"us-diet-pescatarian","label":"pescatarian","field":"dietary","aliases":["fish only","seafood only"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-gluten-free"}}
{"id":"us-diet-gluten-free","label":"gluten free","field":"dietary","aliases":["gf","celiac","no gluten","gluten-free"],"weight":10,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-low-carb"}}
{"id":"us-diet-low-carb","label":"low carb","field":"dietary","aliases":["keto","low carbohydrate","carb free","ketogenic"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-low-calorie"}}
{"id":"us-diet-low-calorie","label":"low calorie","field":"dietary","aliases":["fit and wholesome","calorie smart","light","diet","healthy"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-high-protein"}}
{"id":"us-diet-high-protein","label":"high protein","field":"dietary","aliases":["protein rich","protein packed","high in protein"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-dairy-free"}}
{"id":"us-diet-dairy-free","label":"dairy free","field":"dietary","aliases":["no dairy","lactose free","without dairy"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-nut-free"}}
{"id":"us-diet-nut-free","label":"nut free","field":"dietary","aliases":["no nuts","peanut free","without nuts"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-family-friendly"}}
{"id":"us-diet-family-friendly","label":"family friendly","field":"dietary","aliases":["kid friendly","family","for kids","picky eater"],"weight":9,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-mediterranean"}}
{"id":"us-diet-mediterranean","label":"mediterranean diet","field":"dietary","aliases":["med diet","clean eating"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-diet-meal-prep"}}
{"id":"us-diet-meal-prep","label":"meal prep","field":"dietary","aliases":["prep ahead","make ahead","batch cooking"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-grilled"}}
{"id":"us-method-grilled","label":"grilled","field":"cooking_method","aliases":["grilling","barbecued","bbq","chargrilled","grill"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-baked"}}
{"id":"us-method-baked","label":"baked","field":"cooking_method","aliases":["oven baked","roasted","oven roasted"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-pan-seared"}}
{"id":"us-method-pan-seared","label":"pan seared","field":"cooking_method","aliases":["pan-seared","seared","pan fried"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-fried"}}
{"id":"us-method-fried","label":"fried","field":"cooking_method","aliases":["deep fried","shallow fried","crispy"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-crusted"}}
{"id":"us-method-crusted","label":"crusted","field":"cooking_method","aliases":["parmesan crusted","mozzarella crusted","panko crusted","breaded"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-butter-basted"}}
{"id":"us-method-butter-basted","label":"butter basted","field":"cooking_method","aliases":["butter-basted","basted"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-glazed"}}
{"id":"us-method-glazed","label":"glazed","field":"cooking_method","aliases":["honey glazed","maple glazed","balsamic glazed","teriyaki glazed"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-one-pan"}}
{"id":"us-method-one-pan","label":"one pan","field":"cooking_method","aliases":["one pot","sheet pan","skillet","one skillet"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-quick"}}
{"id":"us-method-quick","label":"quick","field":"cooking_method","aliases":["rapid","20 minute","speedy","fast","easy","quick and easy"],"weight":8,"locale":"en_US","market":"US"}
{"index":{"_id":"us-method-gourmet"}}
{"id":"us-method-gourmet","label":"gourmet","field":"cooking_method","aliases":["date night","special occasion","premium","hall of fame"],"weight":8,"locale":"en_US","market":"US"}
NDJSON

bulk_index "concepts_en_us" "${TMPDIR}/concepts_en_us.ndjson"

# ============================================================================
# 5. LINGUISTIC — GB Synonyms (SYN), Hypernyms (HYP), Stop Words (SW)
# ============================================================================
echo "==> Indexing GB linguistic entries..."

cat > "${TMPDIR}/linguistic_en_gb.ndjson" << 'NDJSON'
{"index":{}}
{"term":"chicken","variant":"poultry","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"poultry","variant":"chicken","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"beef","variant":"cow","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"pork","variant":"pig","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"prawn","variant":"shrimp","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"shrimp","variant":"prawn","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"prawns","variant":"shrimp","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"aubergine","variant":"eggplant","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"eggplant","variant":"aubergine","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"courgette","variant":"zucchini","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"zucchini","variant":"courgette","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"coriander","variant":"cilantro","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"cilantro","variant":"coriander","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"rocket","variant":"arugula","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"arugula","variant":"rocket","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"pak choi","variant":"bok choy","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"bok choy","variant":"pak choi","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"bok choi","variant":"pak choi","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"chips","variant":"fries","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"fries","variant":"chips","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"crisps","variant":"chips","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"mince","variant":"ground meat","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"ground beef","variant":"minced beef","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"minced beef","variant":"ground beef","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"ground lamb","variant":"lamb mince","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"lamb mince","variant":"ground lamb","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"ground pork","variant":"pork mince","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"sneakers","variant":"trainers","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"trainers","variant":"sneakers","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"veggie","variant":"vegetarian","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"vegetarian","variant":"veggie","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"veg","variant":"vegetarian","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"coke","variant":"coca cola","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"mayo","variant":"mayonnaise","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"mayonnaise","variant":"mayo","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"aioli","variant":"garlic mayonnaise","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"yogurt","variant":"yoghurt","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"yoghurt","variant":"yogurt","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"bbq","variant":"barbecue","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"barbecue","variant":"bbq","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"stir fry","variant":"stirfry","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"stirfry","variant":"stir fry","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"spag bol","variant":"spaghetti bolognese","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"ragu","variant":"bolognese","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"bolognese","variant":"ragu","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"hotpot","variant":"casserole","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"casserole","variant":"hotpot","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"ramen","variant":"noodle soup","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"pho","variant":"noodle soup","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"laksa","variant":"noodle soup","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"soya sauce","variant":"soy sauce","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"soy sauce","variant":"soya sauce","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"tamari","variant":"soy sauce","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"shoyu","variant":"soy sauce","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"lasagna","variant":"lasagne","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"lasagne","variant":"lasagna","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"tortilla","variant":"wrap","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"wrap","variant":"tortilla","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"hummus","variant":"houmous","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"houmous","variant":"hummus","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"humous","variant":"houmous","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"naan","variant":"naan bread","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"pitta","variant":"pita","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"pita","variant":"pitta","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"flatbread","variant":"naan","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"parmigiano","variant":"parmesan","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"parmesan","variant":"parmigiano","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"parmigiano reggiano","variant":"parmesan","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"grana padano","variant":"parmesan","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"feta","variant":"greek cheese","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"greek cheese","variant":"feta","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"creme fraiche","variant":"soured cream","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"soured cream","variant":"creme fraiche","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"cottage pie","variant":"shepherds pie","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"shepherds pie","variant":"cottage pie","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"banger","variant":"sausage","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"sausage","variant":"banger","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"dhal","variant":"dal","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"dal","variant":"dhal","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"daal","variant":"dal","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"gyoza","variant":"dumpling","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"dumpling","variant":"gyoza","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"potsticker","variant":"gyoza","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"beetroot","variant":"beet","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"beet","variant":"beetroot","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"sweetcorn","variant":"corn","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"corn","variant":"sweetcorn","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"mange tout","variant":"sugar snap peas","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"sugar snap peas","variant":"mange tout","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"spring onion","variant":"scallion","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"scallion","variant":"spring onion","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"green onion","variant":"spring onion","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"chilli","variant":"chili","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"chili","variant":"chilli","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"capsicum","variant":"pepper","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"bell pepper","variant":"pepper","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"garam masala","variant":"curry powder","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"tahini","variant":"sesame paste","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"sesame paste","variant":"tahini","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"keto","variant":"low carb","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"low carb","variant":"keto","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"plant based","variant":"vegan","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"vegan","variant":"plant based","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"gluten free","variant":"coeliac","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"coeliac","variant":"gluten free","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"celiac","variant":"gluten free","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"tex mex","variant":"mexican","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"confit","variant":"slow cooked","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"crockpot","variant":"slow cooker","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"slow cooker","variant":"crockpot","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"risoni","variant":"orzo","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"orzo","variant":"risoni","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"garbanzo","variant":"chickpeas","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"chickpeas","variant":"garbanzo","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"lima beans","variant":"butter beans","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"butter beans","variant":"lima beans","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"paneer","variant":"indian cheese","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"halloumi","variant":"grilling cheese","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"bean curd","variant":"tofu","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"tofu","variant":"bean curd","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"naanizza","variant":"naan pizza","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"calzone","variant":"folded pizza","type":"SYN","locale":"en_GB"}
{"index":{}}
{"term":"chicken","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"beef","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"pork","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"lamb","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"duck","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"bacon","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"chorizo","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"sausage","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"gammon","variant":"meat","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"salmon","variant":"fish","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"cod","variant":"fish","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"tuna","variant":"fish","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"sea bream","variant":"fish","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"basa","variant":"fish","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"barramundi","variant":"fish","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"prawns","variant":"seafood","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"crab","variant":"seafood","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"penne","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"spaghetti","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"fusilli","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"linguine","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"orzo","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"rigatoni","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"macaroni","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"ravioli","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"gnocchi","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"ditali","variant":"pasta","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"basmati","variant":"rice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"jasmine rice","variant":"rice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"sticky rice","variant":"rice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"udon","variant":"noodles","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"vermicelli","variant":"noodles","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"egg noodles","variant":"noodles","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"rice noodles","variant":"noodles","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"cheddar","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"mozzarella","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"parmesan","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"halloumi","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"burrata","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"feta","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"goats cheese","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"ricotta","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"blue cheese","variant":"cheese","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"ciabatta","variant":"bread","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"sourdough","variant":"bread","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"brioche","variant":"bread","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"focaccia","variant":"bread","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"naan","variant":"bread","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"pitta","variant":"bread","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"tikka masala","variant":"curry","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"korma","variant":"curry","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"thai green curry","variant":"curry","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"dal","variant":"curry","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"bolognese","variant":"pasta sauce","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"carbonara","variant":"pasta sauce","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"alfredo","variant":"pasta sauce","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"pesto","variant":"pasta sauce","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"tacos","variant":"mexican food","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"burrito","variant":"mexican food","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"fajitas","variant":"mexican food","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"enchiladas","variant":"mexican food","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"quesadilla","variant":"mexican food","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"nachos","variant":"mexican food","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"lentils","variant":"legumes","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"chickpeas","variant":"legumes","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"butter beans","variant":"legumes","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"kidney beans","variant":"legumes","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"broccoli","variant":"vegetable","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"cauliflower","variant":"vegetable","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"spinach","variant":"vegetable","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"courgette","variant":"vegetable","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"aubergine","variant":"vegetable","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"asparagus","variant":"vegetable","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"cumin","variant":"spice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"paprika","variant":"spice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"turmeric","variant":"spice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"garam masala","variant":"spice","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"harissa","variant":"spice paste","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"gochujang","variant":"spice paste","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"chermoula","variant":"spice paste","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"miso","variant":"spice paste","type":"HYP","locale":"en_GB"}
{"index":{}}
{"term":"the","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"a","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"an","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"and","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"or","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"with","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"for","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"in","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"of","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"to","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"is","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"it","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"at","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"on","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"by","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"my","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"me","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"i","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"some","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"any","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"all","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"this","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"that","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"recipe","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"recipes","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"meal","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"meals","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"dinner","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"lunch","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"breakfast","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"supper","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"food","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"dish","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"dishes","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"make","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"cook","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"cooking","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"please","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"want","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"need","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"find","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"show","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"get","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"give","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"search","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"looking","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"something","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"tonight","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"today","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"ideas","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"idea","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"style","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"inspired","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"type","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"kind","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"like","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"good","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"best","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"nice","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"really","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"very","variant":"","type":"SW","locale":"en_GB"}
{"index":{}}
{"term":"icecream","variant":"ice cream","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"peanutbutter","variant":"peanut butter","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"crewneck","variant":"crew neck","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"lunchbox","variant":"lunch box","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"toothpaste","variant":"tooth paste","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"milkshake","variant":"milk shake","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"cheesecake","variant":"cheese cake","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"pancake","variant":"pan cake","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"meatball","variant":"meat ball","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"cornflakes","variant":"corn flakes","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"breadcrumbs","variant":"bread crumbs","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"grapefruit","variant":"grape fruit","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"butterscotch","variant":"butter scotch","type":"CMP","locale":"en_GB"}
{"index":{}}
{"term":"shortbread","variant":"short bread","type":"CMP","locale":"en_GB"}
NDJSON

bulk_index "linguistic_en_gb" "${TMPDIR}/linguistic_en_gb.ndjson"

# ============================================================================
# 5b. LINGUISTIC — US market (en_US)
#     US English spelling & terminology differences from UK
# ============================================================================
echo "==> Indexing US linguistic entries..."

cat > "${TMPDIR}/linguistic_en_us.ndjson" << 'NDJSON'
{"index":{}}
{"term":"chicken","variant":"poultry","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"poultry","variant":"chicken","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"beef","variant":"cow","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pork","variant":"pig","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"shrimp","variant":"prawn","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"prawn","variant":"shrimp","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"zucchini","variant":"courgette","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"courgette","variant":"zucchini","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"eggplant","variant":"aubergine","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"aubergine","variant":"eggplant","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"cilantro","variant":"coriander","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"coriander","variant":"cilantro","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"arugula","variant":"rocket","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"rocket","variant":"arugula","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"scallion","variant":"green onion","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"green onion","variant":"scallion","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"spring onion","variant":"scallion","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"bok choy","variant":"pak choi","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pak choi","variant":"bok choy","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"fries","variant":"chips","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"chips","variant":"fries","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ground beef","variant":"minced beef","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"minced beef","variant":"ground beef","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ground turkey","variant":"minced turkey","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ground pork","variant":"minced pork","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ground chicken","variant":"minced chicken","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"hamburger","variant":"burger","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"burger","variant":"hamburger","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"veggie","variant":"vegetarian","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"vegetarian","variant":"veggie","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"bbq","variant":"barbecue","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"barbecue","variant":"bbq","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"stir fry","variant":"stirfry","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"stirfry","variant":"stir fry","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ragu","variant":"bolognese","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"bolognese","variant":"ragu","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"soy sauce","variant":"soya sauce","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"tamari","variant":"soy sauce","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"shoyu","variant":"soy sauce","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"lasagna","variant":"lasagne","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"lasagne","variant":"lasagna","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"tortilla","variant":"wrap","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"wrap","variant":"tortilla","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"hummus","variant":"humous","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"parm","variant":"parmesan","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"parmesan","variant":"parm","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"parmigiano","variant":"parmesan","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pepper jack","variant":"monterey jack","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"monterey jack","variant":"pepper jack","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"sour cream","variant":"creme fraiche","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"creme fraiche","variant":"sour cream","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"filet mignon","variant":"beef tenderloin","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"beef tenderloin","variant":"filet mignon","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ny strip","variant":"new york strip","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"new york strip","variant":"ny strip","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"strip steak","variant":"new york strip","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"flank steak","variant":"bavette","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"bavette","variant":"flank steak","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"carnitas","variant":"pulled pork","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pulled pork","variant":"carnitas","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"mac and cheese","variant":"macaroni and cheese","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"macaroni and cheese","variant":"mac and cheese","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"mac n cheese","variant":"mac and cheese","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"chicken parm","variant":"chicken parmesan","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"chicken parmesan","variant":"chicken parm","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"chicken parmigiana","variant":"chicken parmesan","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pico","variant":"pico de gallo","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pico de gallo","variant":"salsa","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"salsa verde","variant":"green salsa","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"ramen","variant":"noodle soup","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"pho","variant":"noodle soup","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"grits","variant":"polenta","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"polenta","variant":"grits","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"cremini","variant":"baby bella","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"baby bella","variant":"cremini","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"button mushroom","variant":"cremini","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"keto","variant":"low carb","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"low carb","variant":"keto","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"plant based","variant":"vegan","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"vegan","variant":"plant based","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"celiac","variant":"gluten free","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"gluten free","variant":"celiac","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"tex mex","variant":"mexican","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"southwest","variant":"tex mex","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"garbanzo","variant":"chickpeas","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"chickpeas","variant":"garbanzo","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"bean curd","variant":"tofu","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"tofu","variant":"bean curd","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"fettuccine","variant":"tagliatelle","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"tagliatelle","variant":"fettuccine","type":"SYN","locale":"en_US"}
{"index":{}}
{"term":"chicken","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"beef","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"pork","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"lamb","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"turkey","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"bacon","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"sausage","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"pancetta","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"prosciutto","variant":"meat","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"salmon","variant":"fish","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"cod","variant":"fish","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"tuna","variant":"fish","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"tilapia","variant":"fish","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"trout","variant":"fish","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"sea bream","variant":"fish","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"shrimp","variant":"seafood","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"scallops","variant":"seafood","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"lobster","variant":"seafood","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"spaghetti","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"penne","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"rigatoni","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"orzo","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"cavatappi","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"tagliatelle","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"fettuccine","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"gemelli","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"tortelloni","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"ravioli","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"agnolotti","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"gnocchi","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"macaroni","variant":"pasta","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"jasmine rice","variant":"rice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"basmati","variant":"rice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"arborio","variant":"rice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"cheddar","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"mozzarella","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"parmesan","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"monterey jack","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"gouda","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"brie","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"feta","variant":"cheese","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"tacos","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"burrito","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"fajitas","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"enchiladas","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"quesadilla","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"tostada","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"nachos","variant":"mexican food","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"bolognese","variant":"pasta sauce","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"carbonara","variant":"pasta sauce","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"alfredo","variant":"pasta sauce","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"marinara","variant":"pasta sauce","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"pesto","variant":"pasta sauce","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"chickpeas","variant":"legumes","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"black beans","variant":"legumes","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"lentils","variant":"legumes","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"broccoli","variant":"vegetable","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"cauliflower","variant":"vegetable","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"spinach","variant":"vegetable","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"zucchini","variant":"vegetable","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"asparagus","variant":"vegetable","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"kale","variant":"vegetable","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"cumin","variant":"spice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"paprika","variant":"spice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"chipotle","variant":"spice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"old bay","variant":"spice","type":"HYP","locale":"en_US"}
{"index":{}}
{"term":"the","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"a","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"an","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"and","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"or","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"with","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"for","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"in","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"of","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"to","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"is","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"it","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"at","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"on","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"by","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"my","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"me","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"i","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"some","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"any","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"all","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"this","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"that","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"recipe","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"recipes","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"meal","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"meals","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"dinner","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"lunch","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"breakfast","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"supper","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"food","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"dish","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"dishes","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"make","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"cook","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"cooking","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"please","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"want","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"need","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"find","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"show","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"get","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"give","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"search","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"looking","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"something","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"tonight","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"today","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"ideas","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"idea","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"style","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"inspired","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"type","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"kind","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"like","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"good","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"best","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"nice","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"really","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"very","variant":"","type":"SW","locale":"en_US"}
{"index":{}}
{"term":"icecream","variant":"ice cream","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"peanutbutter","variant":"peanut butter","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"cheesecake","variant":"cheese cake","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"milkshake","variant":"milk shake","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"pancake","variant":"pan cake","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"meatball","variant":"meat ball","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"cornflakes","variant":"corn flakes","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"breadcrumbs","variant":"bread crumbs","type":"CMP","locale":"en_US"}
{"index":{}}
{"term":"grapefruit","variant":"grape fruit","type":"CMP","locale":"en_US"}
NDJSON

bulk_index "linguistic_en_us" "${TMPDIR}/linguistic_en_us.ndjson"

# ============================================================================
# 6. CONCEPTS — CA market, en_CA locale
#    Sources: hellofresh.ca/menus, hellofresh.ca/recipes
#    Canadian English — blend of US/UK terms with Canadian specifics
# ============================================================================
echo "==> Indexing CA (en_CA) concepts..."

cat > "${TMPDIR}/concepts_en_ca.ndjson" << 'NDJSON'
{"index":{"_id":"ca-cat-chicken"}}
{"id":"ca-cat-chicken","label":"chicken","field":"category","aliases":["poultry"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chicken-breast"}}
{"id":"ca-cat-chicken-breast","label":"chicken breast","field":"category","aliases":["chicken cutlet","boneless skinless chicken breast","chicken fillet"],"weight":12,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chicken-thigh"}}
{"id":"ca-cat-chicken-thigh","label":"chicken thigh","field":"category","aliases":["boneless chicken thigh","dark meat","chicken thighs"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chicken-drumstick"}}
{"id":"ca-cat-chicken-drumstick","label":"chicken drumstick","field":"category","aliases":["drumstick","chicken leg","chicken drumsticks"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chicken-wing"}}
{"id":"ca-cat-chicken-wing","label":"chicken wing","field":"category","aliases":["chicken wings","buffalo wings","hot wings"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-organic-chicken"}}
{"id":"ca-cat-organic-chicken","label":"organic chicken","field":"category","aliases":["free range chicken","organic poultry"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-beef"}}
{"id":"ca-cat-beef","label":"beef","field":"category","aliases":["cow","bovine"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-ground-beef"}}
{"id":"ca-cat-ground-beef","label":"ground beef","field":"category","aliases":["minced beef","beef mince","lean ground beef"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-striploin-steak"}}
{"id":"ca-cat-striploin-steak","label":"striploin steak","field":"category","aliases":["strip steak","new york strip","sirloin steak","steak"],"weight":12,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-tenderloin-steak"}}
{"id":"ca-cat-tenderloin-steak","label":"tenderloin steak","field":"category","aliases":["filet mignon","beef tenderloin","fillet steak"],"weight":12,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-beef-meatball"}}
{"id":"ca-cat-beef-meatball","label":"beef meatballs","field":"category","aliases":["meatballs","beef meatball"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-pork"}}
{"id":"ca-cat-pork","label":"pork","field":"category","aliases":["pig"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-pork-chop"}}
{"id":"ca-cat-pork-chop","label":"pork chop","field":"category","aliases":["bone-in pork chop","pork chops","pork cutlet"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-italian-sausage"}}
{"id":"ca-cat-italian-sausage","label":"italian sausage","field":"category","aliases":["sausage","pork sausage","italian pork sausage"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chorizo"}}
{"id":"ca-cat-chorizo","label":"chorizo","field":"category","aliases":["spanish sausage","chorizo sausage"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-bacon"}}
{"id":"ca-cat-bacon","label":"bacon","field":"category","aliases":["peameal bacon","back bacon","canadian bacon","bacon rashers"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-turkey"}}
{"id":"ca-cat-turkey","label":"turkey","field":"category","aliases":["ground turkey","turkey breast"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-ground-turkey"}}
{"id":"ca-cat-ground-turkey","label":"ground turkey","field":"category","aliases":["minced turkey","turkey mince"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-duck"}}
{"id":"ca-cat-duck","label":"duck","field":"category","aliases":["duck breast","duck leg","confit duck"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-salmon"}}
{"id":"ca-cat-salmon","label":"salmon","field":"category","aliases":["atlantic salmon","salmon fillet","pacific salmon","wild salmon"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-shrimp"}}
{"id":"ca-cat-shrimp","label":"shrimp","field":"category","aliases":["prawns","jumbo shrimp","tiger shrimp"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-tilapia"}}
{"id":"ca-cat-tilapia","label":"tilapia","field":"category","aliases":["white fish","tilapia fillet"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-scallops"}}
{"id":"ca-cat-scallops","label":"scallops","field":"category","aliases":["sea scallops","bay scallops"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-tofu"}}
{"id":"ca-cat-tofu","label":"tofu","field":"category","aliases":["bean curd","firm tofu","silken tofu"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-halloumi"}}
{"id":"ca-cat-halloumi","label":"halloumi","field":"category","aliases":["halloumi cheese","grilling cheese"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-paneer"}}
{"id":"ca-cat-paneer","label":"paneer","field":"category","aliases":["indian cheese","paneer cheese"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-beyond-meat"}}
{"id":"ca-cat-beyond-meat","label":"beyond meat","field":"category","aliases":["plant-based meat","beyond burger","meat alternative"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-potato"}}
{"id":"ca-cat-potato","label":"potato","field":"ingredient","aliases":["potatoes","baby potato","roasting potato","yukon gold"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-sweet-potato"}}
{"id":"ca-cat-sweet-potato","label":"sweet potato","field":"ingredient","aliases":["sweet potatoes","yam"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-broccoli"}}
{"id":"ca-cat-broccoli","label":"broccoli","field":"ingredient","aliases":["broccoli florets","tenderstem broccoli"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-cauliflower"}}
{"id":"ca-cat-cauliflower","label":"cauliflower","field":"ingredient","aliases":["cauliflower florets"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-spinach"}}
{"id":"ca-cat-spinach","label":"spinach","field":"ingredient","aliases":["baby spinach"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-kale"}}
{"id":"ca-cat-kale","label":"kale","field":"ingredient","aliases":["curly kale","tuscan kale","lacinato kale"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-zucchini"}}
{"id":"ca-cat-zucchini","label":"zucchini","field":"ingredient","aliases":["courgette","zucchinis"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-mushroom"}}
{"id":"ca-cat-mushroom","label":"mushroom","field":"ingredient","aliases":["mushrooms","cremini","button mushroom","portobello"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-pepper"}}
{"id":"ca-cat-pepper","label":"pepper","field":"ingredient","aliases":["bell pepper","sweet pepper","red pepper","green pepper"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-corn"}}
{"id":"ca-cat-corn","label":"corn","field":"ingredient","aliases":["sweetcorn","corn on the cob","peace arch corn"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-bok-choy"}}
{"id":"ca-cat-bok-choy","label":"bok choy","field":"ingredient","aliases":["pak choi","baby bok choy","chinese cabbage"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-edamame"}}
{"id":"ca-cat-edamame","label":"edamame","field":"ingredient","aliases":["soybeans","soy beans"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-brussels-sprouts"}}
{"id":"ca-cat-brussels-sprouts","label":"brussels sprouts","field":"ingredient","aliases":["sprouts","brussel sprouts"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-avocado"}}
{"id":"ca-cat-avocado","label":"avocado","field":"ingredient","aliases":["avo","guacamole"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chickpeas"}}
{"id":"ca-cat-chickpeas","label":"chickpeas","field":"ingredient","aliases":["garbanzo beans","chick peas"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-black-beans"}}
{"id":"ca-cat-black-beans","label":"black beans","field":"ingredient","aliases":["turtle beans"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-lentils"}}
{"id":"ca-cat-lentils","label":"lentils","field":"ingredient","aliases":["red lentils","green lentils","dal"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-feta"}}
{"id":"ca-cat-feta","label":"feta","field":"ingredient","aliases":["feta cheese","crumbled feta"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-cheddar"}}
{"id":"ca-cat-cheddar","label":"cheddar","field":"ingredient","aliases":["cheddar cheese","sharp cheddar","old cheddar"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-parmesan"}}
{"id":"ca-cat-parmesan","label":"parmesan","field":"ingredient","aliases":["parmigiano","parmesan cheese","parm"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-rice"}}
{"id":"ca-cat-rice","label":"rice","field":"ingredient","aliases":["jasmine rice","basmati rice","wild rice","long grain rice"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-pasta"}}
{"id":"ca-cat-pasta","label":"pasta","field":"ingredient","aliases":["noodles","penne","fusilli","rigatoni","linguine","spaghetti"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-gnocchi"}}
{"id":"ca-cat-gnocchi","label":"gnocchi","field":"ingredient","aliases":["potato gnocchi"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-tortellini"}}
{"id":"ca-cat-tortellini","label":"tortellini","field":"ingredient","aliases":["cheese tortellini"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-couscous"}}
{"id":"ca-cat-couscous","label":"couscous","field":"ingredient","aliases":["pearl couscous","israeli couscous"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-bulgur"}}
{"id":"ca-cat-bulgur","label":"bulgur","field":"ingredient","aliases":["bulgur wheat","cracked wheat"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-farro"}}
{"id":"ca-cat-farro","label":"farro","field":"ingredient","aliases":["emmer wheat"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-flatbread"}}
{"id":"ca-cat-flatbread","label":"flatbread","field":"ingredient","aliases":["naan","pita","tortilla","wrap"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-maple-syrup"}}
{"id":"ca-cat-maple-syrup","label":"maple syrup","field":"ingredient","aliases":["maple","sirop d'erable","pure maple syrup"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-poutine"}}
{"id":"ca-cat-poutine","label":"poutine","field":"category","aliases":["loaded poutine","fries and gravy","cheese curds and fries"],"weight":11,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-peameal-bacon"}}
{"id":"ca-cat-peameal-bacon","label":"peameal bacon","field":"category","aliases":["canadian bacon","back bacon","cornmeal bacon"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-butter-tart"}}
{"id":"ca-cat-butter-tart","label":"butter tart","field":"category","aliases":["butter tarts","canadian tart"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-nanaimo-bar"}}
{"id":"ca-cat-nanaimo-bar","label":"nanaimo bar","field":"category","aliases":["nanaimo bars","chocolate bar dessert"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-tourtiere"}}
{"id":"ca-cat-tourtiere","label":"tourtiere","field":"category","aliases":["meat pie","quebec meat pie","french canadian pie"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-italian"}}
{"id":"ca-cuisine-italian","label":"italian","field":"cuisine","aliases":["italia"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-mexican"}}
{"id":"ca-cuisine-mexican","label":"mexican","field":"cuisine","aliases":["tex-mex","texmex","latin"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-japanese"}}
{"id":"ca-cuisine-japanese","label":"japanese","field":"cuisine","aliases":["japan"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-korean"}}
{"id":"ca-cuisine-korean","label":"korean","field":"cuisine","aliases":["korea"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-thai"}}
{"id":"ca-cuisine-thai","label":"thai","field":"cuisine","aliases":["thailand"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-chinese"}}
{"id":"ca-cuisine-chinese","label":"chinese","field":"cuisine","aliases":["china","cantonese","szechuan"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-vietnamese"}}
{"id":"ca-cuisine-vietnamese","label":"vietnamese","field":"cuisine","aliases":["vietnam","viet"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-indian"}}
{"id":"ca-cuisine-indian","label":"indian","field":"cuisine","aliases":["india","curry","north indian"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-mediterranean"}}
{"id":"ca-cuisine-mediterranean","label":"mediterranean","field":"cuisine","aliases":["med","greek style"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-greek"}}
{"id":"ca-cuisine-greek","label":"greek","field":"cuisine","aliases":["greece","gyro","souvlaki"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-middle-eastern"}}
{"id":"ca-cuisine-middle-eastern","label":"middle eastern","field":"cuisine","aliases":["lebanese","turkish","persian","falafel"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-caribbean"}}
{"id":"ca-cuisine-caribbean","label":"caribbean","field":"cuisine","aliases":["jamaican","island","jerk"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-french"}}
{"id":"ca-cuisine-french","label":"french","field":"cuisine","aliases":["france","bistro","provencal","quebecois"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-moroccan"}}
{"id":"ca-cuisine-moroccan","label":"moroccan","field":"cuisine","aliases":["morocco","north african","tagine"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-peruvian"}}
{"id":"ca-cuisine-peruvian","label":"peruvian","field":"cuisine","aliases":["peru","lomo saltado"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-swedish"}}
{"id":"ca-cuisine-swedish","label":"swedish","field":"cuisine","aliases":["scandinavian","nordic"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cuisine-canadian"}}
{"id":"ca-cuisine-canadian","label":"canadian","field":"cuisine","aliases":["canada","quebecois","maple"],"weight":10,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-vegetarian"}}
{"id":"ca-diet-vegetarian","label":"vegetarian","field":"dietary","aliases":["veggie","veg","meat-free","meatless"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-vegan"}}
{"id":"ca-diet-vegan","label":"vegan","field":"dietary","aliases":["plant-based","plant based","dairy-free"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-calorie-smart"}}
{"id":"ca-diet-calorie-smart","label":"calorie smart","field":"dietary","aliases":["cal smart","under 650 calories","low calorie","light"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-carb-smart"}}
{"id":"ca-diet-carb-smart","label":"carb smart","field":"dietary","aliases":["low carb","under 50g carbs","keto friendly"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-high-protein"}}
{"id":"ca-diet-high-protein","label":"high protein","field":"dietary","aliases":["protein rich","high-protein","power up"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-gluten-free"}}
{"id":"ca-diet-gluten-free","label":"gluten free","field":"dietary","aliases":["gf","no gluten","gluten-free","celiac friendly"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-dairy-free"}}
{"id":"ca-diet-dairy-free","label":"dairy free","field":"dietary","aliases":["no dairy","dairy-free","lactose free"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-nut-free"}}
{"id":"ca-diet-nut-free","label":"nut free","field":"dietary","aliases":["no nuts","nut-free","peanut free"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-family-friendly"}}
{"id":"ca-diet-family-friendly","label":"family friendly","field":"dietary","aliases":["kid friendly","kids","family"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-diet-balanced"}}
{"id":"ca-diet-balanced","label":"balanced","field":"dietary","aliases":["nutritionist pick","healthy","wholesome"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-burger"}}
{"id":"ca-meal-burger","label":"burger","field":"meal_type","aliases":["burgers","hamburger","cheeseburger"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-taco"}}
{"id":"ca-meal-taco","label":"taco","field":"meal_type","aliases":["tacos","tostada","tostadas"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-stir-fry"}}
{"id":"ca-meal-stir-fry","label":"stir fry","field":"meal_type","aliases":["stir-fry","stirfry","wok"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-curry"}}
{"id":"ca-meal-curry","label":"curry","field":"meal_type","aliases":["curries","tikka masala","butter chicken"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-pasta-dish"}}
{"id":"ca-meal-pasta-dish","label":"pasta","field":"meal_type","aliases":["spaghetti","penne","fettuccine","linguine"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-bowl"}}
{"id":"ca-meal-bowl","label":"bowl","field":"meal_type","aliases":["bowls","rice bowl","grain bowl","donburi","bibimbap"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-wrap"}}
{"id":"ca-meal-wrap","label":"wrap","field":"meal_type","aliases":["wraps","gyro","burrito","kofta wrap"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-soup"}}
{"id":"ca-meal-soup","label":"soup","field":"meal_type","aliases":["soups","stew","chowder","bisque"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-salad"}}
{"id":"ca-meal-salad","label":"salad","field":"meal_type","aliases":["salads","side salad"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-sandwich"}}
{"id":"ca-meal-sandwich","label":"sandwich","field":"meal_type","aliases":["sandwiches","sub","hoagie","panini"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-pizza"}}
{"id":"ca-meal-pizza","label":"pizza","field":"meal_type","aliases":["flatbread pizza","naan pizza"],"weight":9,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-meal-skillet"}}
{"id":"ca-meal-skillet","label":"skillet","field":"meal_type","aliases":["one pan","one-pan","sheet pan"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-quick"}}
{"id":"ca-method-quick","label":"quick","field":"cooking_method","aliases":["fast","easy","15 minute","20 minute","superquick","speedy"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-one-pan"}}
{"id":"ca-method-one-pan","label":"one pan","field":"cooking_method","aliases":["one-pan","one pot","one-pot","sheet pan"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-grilled"}}
{"id":"ca-method-grilled","label":"grilled","field":"cooking_method","aliases":["grill","bbq","barbecue","charred"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-baked"}}
{"id":"ca-method-baked","label":"baked","field":"cooking_method","aliases":["oven","roasted","oven-baked"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-pan-fried"}}
{"id":"ca-method-pan-fried","label":"pan fried","field":"cooking_method","aliases":["pan-fried","seared","pan-seared","crispy"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-stir-fried"}}
{"id":"ca-method-stir-fried","label":"stir fried","field":"cooking_method","aliases":["stir-fried","wok-fried"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-braised"}}
{"id":"ca-method-braised","label":"braised","field":"cooking_method","aliases":["slow-cooked","slow cooked"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-gourmet"}}
{"id":"ca-method-gourmet","label":"gourmet","field":"cooking_method","aliases":["date night","special occasion","premium","deluxe"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-method-big-batch"}}
{"id":"ca-method-big-batch","label":"big batch","field":"cooking_method","aliases":["meal prep","batch cooking","leftovers"],"weight":8,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-kimchi"}}
{"id":"ca-cat-kimchi","label":"kimchi","field":"ingredient","aliases":["korean pickled cabbage"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-hummus"}}
{"id":"ca-cat-hummus","label":"hummus","field":"ingredient","aliases":["houmous","chickpea dip"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-peanut"}}
{"id":"ca-cat-peanut","label":"peanut","field":"ingredient","aliases":["peanuts","peanut butter","peanut sauce"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-teriyaki"}}
{"id":"ca-cat-teriyaki","label":"teriyaki","field":"ingredient","aliases":["teriyaki sauce","teriyaki glaze"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-chipotle"}}
{"id":"ca-cat-chipotle","label":"chipotle","field":"ingredient","aliases":["chipotle pepper","chipotle sauce","chipotle crema"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-gochujang"}}
{"id":"ca-cat-gochujang","label":"gochujang","field":"ingredient","aliases":["korean chili paste","korean pepper paste"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-hoisin"}}
{"id":"ca-cat-hoisin","label":"hoisin","field":"ingredient","aliases":["hoisin sauce","chinese bbq sauce"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-miso"}}
{"id":"ca-cat-miso","label":"miso","field":"ingredient","aliases":["miso paste","white miso"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-tahini"}}
{"id":"ca-cat-tahini","label":"tahini","field":"ingredient","aliases":["sesame paste","tahini sauce"],"weight":7,"locale":"en_CA","market":"CA"}
{"index":{"_id":"ca-cat-pesto"}}
{"id":"ca-cat-pesto","label":"pesto","field":"ingredient","aliases":["basil pesto","sun-dried tomato pesto"],"weight":7,"locale":"en_CA","market":"CA"}
NDJSON

bulk_index "concepts_en_ca" "${TMPDIR}/concepts_en_ca.ndjson"

# ============================================================================
# 6b. CONCEPTS — CA market, fr_CA locale
#     French-Canadian food taxonomy from hellofresh.ca/fr
#     Quebec French terms for proteins, ingredients, cuisines, dietary labels
# ============================================================================
echo "==> Indexing CA (fr_CA) concepts..."

cat > "${TMPDIR}/concepts_fr_ca.ndjson" << 'NDJSON'
{"index":{"_id":"fr-cat-poulet"}}
{"id":"fr-cat-poulet","label":"poulet","field":"category","aliases":["volaille","poulet entier"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-poitrine-poulet"}}
{"id":"fr-cat-poitrine-poulet","label":"poitrine de poulet","field":"category","aliases":["escalope de poulet","filet de poulet","blanc de poulet"],"weight":12,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-cuisse-poulet"}}
{"id":"fr-cat-cuisse-poulet","label":"cuisse de poulet","field":"category","aliases":["haut de cuisse","cuisses de poulet"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-pilon-poulet"}}
{"id":"fr-cat-pilon-poulet","label":"pilon de poulet","field":"category","aliases":["pilon","pilons de poulet"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-aile-poulet"}}
{"id":"fr-cat-aile-poulet","label":"aile de poulet","field":"category","aliases":["ailes de poulet","ailes buffalo"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-boeuf"}}
{"id":"fr-cat-boeuf","label":"boeuf","field":"category","aliases":["bœuf"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-boeuf-hache"}}
{"id":"fr-cat-boeuf-hache","label":"boeuf haché","field":"category","aliases":["bœuf haché","viande hachée"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-steak"}}
{"id":"fr-cat-steak","label":"steak","field":"category","aliases":["bifteck","entrecôte","faux-filet","contre-filet"],"weight":12,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-boulettes-boeuf"}}
{"id":"fr-cat-boulettes-boeuf","label":"boulettes de boeuf","field":"category","aliases":["boulettes","boulettes de viande"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-porc"}}
{"id":"fr-cat-porc","label":"porc","field":"category","aliases":["cochon"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-cotelette-porc"}}
{"id":"fr-cat-cotelette-porc","label":"côtelette de porc","field":"category","aliases":["côtelettes de porc","cotelette de porc"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-saucisse"}}
{"id":"fr-cat-saucisse","label":"saucisse","field":"category","aliases":["saucisse italienne","saucisse de porc","saucisses"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-chorizo"}}
{"id":"fr-cat-chorizo","label":"chorizo","field":"category","aliases":["saucisse espagnole"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-bacon"}}
{"id":"fr-cat-bacon","label":"bacon","field":"category","aliases":["lard","bacon fumé","bacon de dos"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-dinde"}}
{"id":"fr-cat-dinde","label":"dinde","field":"category","aliases":["dinde hachée","poitrine de dinde"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-canard"}}
{"id":"fr-cat-canard","label":"canard","field":"category","aliases":["poitrine de canard","cuisse de canard","confit de canard"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-saumon"}}
{"id":"fr-cat-saumon","label":"saumon","field":"category","aliases":["filet de saumon","saumon atlantique","saumon sauvage"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-crevettes"}}
{"id":"fr-cat-crevettes","label":"crevettes","field":"category","aliases":["crevette","gambas","crevettes géantes"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-tilapia"}}
{"id":"fr-cat-tilapia","label":"tilapia","field":"category","aliases":["poisson blanc","filet de tilapia"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-petoncles"}}
{"id":"fr-cat-petoncles","label":"pétoncles","field":"category","aliases":["coquilles saint-jacques"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-tofu"}}
{"id":"fr-cat-tofu","label":"tofu","field":"category","aliases":["tofu ferme","tofu soyeux"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-halloumi"}}
{"id":"fr-cat-halloumi","label":"halloumi","field":"category","aliases":["fromage halloumi","fromage grillé"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-paneer"}}
{"id":"fr-cat-paneer","label":"paneer","field":"category","aliases":["fromage indien"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-pomme-de-terre"}}
{"id":"fr-cat-pomme-de-terre","label":"pomme de terre","field":"ingredient","aliases":["patate","pommes de terre","patates"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-patate-douce"}}
{"id":"fr-cat-patate-douce","label":"patate douce","field":"ingredient","aliases":["patates douces","igname"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-brocoli"}}
{"id":"fr-cat-brocoli","label":"brocoli","field":"ingredient","aliases":["fleurettes de brocoli"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-chou-fleur"}}
{"id":"fr-cat-chou-fleur","label":"chou-fleur","field":"ingredient","aliases":["chou fleur"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-epinards"}}
{"id":"fr-cat-epinards","label":"épinards","field":"ingredient","aliases":["epinards","jeunes épinards"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-courgette"}}
{"id":"fr-cat-courgette","label":"courgette","field":"ingredient","aliases":["courgettes","zucchini"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-champignon"}}
{"id":"fr-cat-champignon","label":"champignon","field":"ingredient","aliases":["champignons","cremini","portobello"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-poivron"}}
{"id":"fr-cat-poivron","label":"poivron","field":"ingredient","aliases":["poivrons","piment doux","poivron rouge","poivron vert"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-mais"}}
{"id":"fr-cat-mais","label":"maïs","field":"ingredient","aliases":["mais","blé d'inde","épi de maïs"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-bok-choy"}}
{"id":"fr-cat-bok-choy","label":"bok choy","field":"ingredient","aliases":["pak choï","chou chinois","bébé bok choy"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-avocat"}}
{"id":"fr-cat-avocat","label":"avocat","field":"ingredient","aliases":["guacamole"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-pois-chiches"}}
{"id":"fr-cat-pois-chiches","label":"pois chiches","field":"ingredient","aliases":["pois chiche"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-haricots-noirs"}}
{"id":"fr-cat-haricots-noirs","label":"haricots noirs","field":"ingredient","aliases":["haricot noir","fèves noires"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-lentilles"}}
{"id":"fr-cat-lentilles","label":"lentilles","field":"ingredient","aliases":["lentilles rouges","lentilles vertes","dal"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-fromage"}}
{"id":"fr-cat-fromage","label":"fromage","field":"ingredient","aliases":["feta","cheddar","parmesan","mozzarella"],"weight":7,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-riz"}}
{"id":"fr-cat-riz","label":"riz","field":"ingredient","aliases":["riz jasmin","riz basmati","riz sauvage"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-pates"}}
{"id":"fr-cat-pates","label":"pâtes","field":"ingredient","aliases":["pates","nouilles","spaghetti","penne","fusilli","linguine","rigatoni"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-gnocchi"}}
{"id":"fr-cat-gnocchi","label":"gnocchi","field":"ingredient","aliases":["gnocchi de pommes de terre"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-couscous"}}
{"id":"fr-cat-couscous","label":"couscous","field":"ingredient","aliases":["couscous perlé"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-sirop-erable"}}
{"id":"fr-cat-sirop-erable","label":"sirop d'érable","field":"ingredient","aliases":["sirop d'erable","érable","maple syrup"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-poutine"}}
{"id":"fr-cat-poutine","label":"poutine","field":"category","aliases":["frites et sauce","fromage en grains"],"weight":11,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-tourtiere"}}
{"id":"fr-cat-tourtiere","label":"tourtière","field":"category","aliases":["pâté à la viande","tourte à la viande"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-bacon-peameal"}}
{"id":"fr-cat-bacon-peameal","label":"bacon peameal","field":"category","aliases":["bacon canadien","bacon de dos"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cat-tarte-au-beurre"}}
{"id":"fr-cat-tarte-au-beurre","label":"tarte au beurre","field":"category","aliases":["tartes au beurre"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-italienne"}}
{"id":"fr-cuisine-italienne","label":"italienne","field":"cuisine","aliases":["italien","italia"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-mexicaine"}}
{"id":"fr-cuisine-mexicaine","label":"mexicaine","field":"cuisine","aliases":["mexicain","tex-mex"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-japonaise"}}
{"id":"fr-cuisine-japonaise","label":"japonaise","field":"cuisine","aliases":["japonais","japon"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-coreenne"}}
{"id":"fr-cuisine-coreenne","label":"coréenne","field":"cuisine","aliases":["coréen","corée"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-thailandaise"}}
{"id":"fr-cuisine-thailandaise","label":"thaïlandaise","field":"cuisine","aliases":["thaïlandais","thaï","thai"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-chinoise"}}
{"id":"fr-cuisine-chinoise","label":"chinoise","field":"cuisine","aliases":["chinois","cantonais","szechuan"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-vietnamienne"}}
{"id":"fr-cuisine-vietnamienne","label":"vietnamienne","field":"cuisine","aliases":["vietnamien","vietnam"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-indienne"}}
{"id":"fr-cuisine-indienne","label":"indienne","field":"cuisine","aliases":["indien","inde","cari","curry"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-mediterraneenne"}}
{"id":"fr-cuisine-mediterraneenne","label":"méditerranéenne","field":"cuisine","aliases":["méditerranéen","grec"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-grecque"}}
{"id":"fr-cuisine-grecque","label":"grecque","field":"cuisine","aliases":["grec","grèce","gyros","souvlaki"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-moyen-orient"}}
{"id":"fr-cuisine-moyen-orient","label":"moyen-orient","field":"cuisine","aliases":["libanaise","turque","perse","falafel"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-caribbeenne"}}
{"id":"fr-cuisine-caribbeenne","label":"caribéenne","field":"cuisine","aliases":["jamaïcaine","antillaise","jerk"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-francaise"}}
{"id":"fr-cuisine-francaise","label":"française","field":"cuisine","aliases":["français","france","bistro","québécoise"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-marocaine"}}
{"id":"fr-cuisine-marocaine","label":"marocaine","field":"cuisine","aliases":["marocain","nord-africaine","tajine"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-cuisine-canadienne"}}
{"id":"fr-cuisine-canadienne","label":"canadienne","field":"cuisine","aliases":["canadien","québécois","érable"],"weight":10,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-vegetarien"}}
{"id":"fr-diet-vegetarien","label":"végétarien","field":"dietary","aliases":["vegetarien","veggie","sans viande"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-vegetalien"}}
{"id":"fr-diet-vegetalien","label":"végétalien","field":"dietary","aliases":["vegetalien","vegan","à base de plantes","sans produits laitiers"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-faible-calories"}}
{"id":"fr-diet-faible-calories","label":"faible en calories","field":"dietary","aliases":["cal smart","léger","moins de 650 calories"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-faible-glucides"}}
{"id":"fr-diet-faible-glucides","label":"faible en glucides","field":"dietary","aliases":["carb smart","peu de glucides","moins de 50g glucides"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-riche-proteines"}}
{"id":"fr-diet-riche-proteines","label":"riche en protéines","field":"dietary","aliases":["haute teneur en protéines","protéiné"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-sans-gluten"}}
{"id":"fr-diet-sans-gluten","label":"sans gluten","field":"dietary","aliases":["gluten free","cœliaque"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-sans-produits-laitiers"}}
{"id":"fr-diet-sans-produits-laitiers","label":"sans produits laitiers","field":"dietary","aliases":["sans lactose","dairy free"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-diet-familial"}}
{"id":"fr-diet-familial","label":"familial","field":"dietary","aliases":["pour la famille","adapté aux enfants"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-hamburger"}}
{"id":"fr-meal-hamburger","label":"hamburger","field":"meal_type","aliases":["burger","burgers","cheeseburger"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-taco"}}
{"id":"fr-meal-taco","label":"taco","field":"meal_type","aliases":["tacos","tostada"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-saute"}}
{"id":"fr-meal-saute","label":"sauté","field":"meal_type","aliases":["saute","sauté au wok","wok"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-cari"}}
{"id":"fr-meal-cari","label":"cari","field":"meal_type","aliases":["curry","tikka masala","poulet au beurre"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-pates"}}
{"id":"fr-meal-pates","label":"pâtes","field":"meal_type","aliases":["pasta","spaghetti","penne","fettuccine"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-bol"}}
{"id":"fr-meal-bol","label":"bol","field":"meal_type","aliases":["bols","bol de riz","bol de grains","donburi","bibimbap"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-wrap"}}
{"id":"fr-meal-wrap","label":"wrap","field":"meal_type","aliases":["wraps","gyro","burrito","kefta"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-soupe"}}
{"id":"fr-meal-soupe","label":"soupe","field":"meal_type","aliases":["soupes","ragoût","chaudrée","potage"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-salade"}}
{"id":"fr-meal-salade","label":"salade","field":"meal_type","aliases":["salades"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-sandwich"}}
{"id":"fr-meal-sandwich","label":"sandwich","field":"meal_type","aliases":["sandwichs","sous-marin","panini"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-meal-pizza"}}
{"id":"fr-meal-pizza","label":"pizza","field":"meal_type","aliases":["pizza naan","pizza maison"],"weight":9,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-rapide"}}
{"id":"fr-method-rapide","label":"rapide","field":"cooking_method","aliases":["vite","facile","15 minutes","20 minutes","super rapide"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-un-seul-poele"}}
{"id":"fr-method-un-seul-poele","label":"un seul poêle","field":"cooking_method","aliases":["un poêle","un chaudron","plaque de cuisson"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-grille"}}
{"id":"fr-method-grille","label":"grillé","field":"cooking_method","aliases":["grille","bbq","barbecue"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-au-four"}}
{"id":"fr-method-au-four","label":"au four","field":"cooking_method","aliases":["rôti","cuit au four"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-poele"}}
{"id":"fr-method-poele","label":"poêlé","field":"cooking_method","aliases":["poele","saisi","croustillant"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-braise"}}
{"id":"fr-method-braise","label":"braisé","field":"cooking_method","aliases":["braise","mijoté","cuisson lente"],"weight":8,"locale":"fr_CA","market":"CA"}
{"index":{"_id":"fr-method-gastronomique"}}
{"id":"fr-method-gastronomique","label":"gastronomique","field":"cooking_method","aliases":["soirée spéciale","gourmet","de luxe"],"weight":8,"locale":"fr_CA","market":"CA"}
NDJSON

bulk_index "concepts_fr_ca" "${TMPDIR}/concepts_fr_ca.ndjson"

# ============================================================================
# 7. LINGUISTIC — CA market, en_CA locale
#    Canadian English synonyms, hypernyms, stop words
# ============================================================================
echo "==> Indexing CA (en_CA) linguistic entries..."

cat > "${TMPDIR}/linguistic_en_ca.ndjson" << 'NDJSON'
{"index":{}}
{"term":"chicken","variant":"poultry","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"poultry","variant":"chicken","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"beef","variant":"cow","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"cow","variant":"beef","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"pork","variant":"pig","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"pig","variant":"pork","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"shrimp","variant":"prawns","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"prawns","variant":"shrimp","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"zucchini","variant":"courgette","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"courgette","variant":"zucchini","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"eggplant","variant":"aubergine","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"aubergine","variant":"eggplant","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"cilantro","variant":"coriander","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"coriander","variant":"cilantro","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"ground beef","variant":"minced beef","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"minced beef","variant":"ground beef","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"chips","variant":"fries","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"fries","variant":"chips","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"french fries","variant":"fries","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"pop","variant":"soda","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"soda","variant":"pop","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"peameal bacon","variant":"canadian bacon","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"canadian bacon","variant":"peameal bacon","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"back bacon","variant":"peameal bacon","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"garbanzo beans","variant":"chickpeas","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"chickpeas","variant":"garbanzo beans","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"bell pepper","variant":"pepper","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"capsicum","variant":"pepper","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"arugula","variant":"rocket","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"rocket","variant":"arugula","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"scallion","variant":"green onion","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"green onion","variant":"scallion","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"spring onion","variant":"green onion","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"icing sugar","variant":"powdered sugar","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"powdered sugar","variant":"icing sugar","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"veggie","variant":"vegetarian","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"vegetarian","variant":"veggie","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"veg","variant":"vegetarian","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"bbq","variant":"barbecue","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"barbecue","variant":"bbq","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"grill","variant":"barbecue","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"coke","variant":"coca cola","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"mac and cheese","variant":"macaroni and cheese","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"kraft dinner","variant":"macaroni and cheese","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"kd","variant":"macaroni and cheese","type":"SYN","locale":"en_CA"}
{"index":{}}
{"term":"chicken","variant":"meat","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"beef","variant":"meat","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"pork","variant":"meat","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"turkey","variant":"meat","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"salmon","variant":"fish","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"tilapia","variant":"fish","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"shrimp","variant":"seafood","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"scallops","variant":"seafood","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"spaghetti","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"penne","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"linguine","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"fusilli","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"rigatoni","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"fettuccine","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"gnocchi","variant":"pasta","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"jasmine rice","variant":"rice","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"basmati rice","variant":"rice","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"wild rice","variant":"rice","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"broccoli","variant":"vegetable","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"spinach","variant":"vegetable","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"kale","variant":"vegetable","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"zucchini","variant":"vegetable","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"cauliflower","variant":"vegetable","type":"HYP","locale":"en_CA"}
{"index":{}}
{"term":"the","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"a","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"an","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"and","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"or","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"with","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"for","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"of","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"in","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"on","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"to","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"some","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"any","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"just","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"really","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"very","variant":"","type":"SW","locale":"en_CA"}
{"index":{}}
{"term":"nice","variant":"","type":"SW","locale":"en_CA"}
NDJSON

bulk_index "linguistic_en_ca" "${TMPDIR}/linguistic_en_ca.ndjson"

# ============================================================================
# 7b. LINGUISTIC — CA market, fr_CA locale
#     French-Canadian synonyms, hypernyms, stop words
# ============================================================================
echo "==> Indexing CA (fr_CA) linguistic entries..."

cat > "${TMPDIR}/linguistic_fr_ca.ndjson" << 'NDJSON'
{"index":{}}
{"term":"poulet","variant":"volaille","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"volaille","variant":"poulet","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"boeuf","variant":"bœuf","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"bœuf","variant":"boeuf","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"porc","variant":"cochon","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"cochon","variant":"porc","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"crevettes","variant":"gambas","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"gambas","variant":"crevettes","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"courgette","variant":"zucchini","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"zucchini","variant":"courgette","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"pomme de terre","variant":"patate","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"patate","variant":"pomme de terre","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"maïs","variant":"blé d'inde","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"blé d'inde","variant":"maïs","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"coriandre","variant":"cilantro","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"cilantro","variant":"coriandre","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"boeuf haché","variant":"viande hachée","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"viande hachée","variant":"boeuf haché","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"frites","variant":"patates frites","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"patates frites","variant":"frites","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"saucisse","variant":"saucisses","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"fromage","variant":"cheese","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"sirop d'érable","variant":"érable","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"érable","variant":"sirop d'érable","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"pâtes","variant":"nouilles","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"nouilles","variant":"pâtes","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"cari","variant":"curry","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"curry","variant":"cari","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"végétarien","variant":"veggie","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"veggie","variant":"végétarien","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"bbq","variant":"barbecue","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"barbecue","variant":"bbq","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"grillé","variant":"barbecue","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"sauté","variant":"poêlé","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"poêlé","variant":"sauté","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"rapide","variant":"vite","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"vite","variant":"rapide","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"facile","variant":"simple","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"simple","variant":"facile","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"épicé","variant":"piquant","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"piquant","variant":"épicé","type":"SYN","locale":"fr_CA"}
{"index":{}}
{"term":"poulet","variant":"viande","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"boeuf","variant":"viande","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"porc","variant":"viande","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"dinde","variant":"viande","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"saumon","variant":"poisson","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"tilapia","variant":"poisson","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"crevettes","variant":"fruits de mer","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"pétoncles","variant":"fruits de mer","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"spaghetti","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"penne","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"linguine","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"fusilli","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"rigatoni","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"fettuccine","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"gnocchi","variant":"pâtes","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"riz jasmin","variant":"riz","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"riz basmati","variant":"riz","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"riz sauvage","variant":"riz","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"brocoli","variant":"légume","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"épinards","variant":"légume","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"courgette","variant":"légume","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"chou-fleur","variant":"légume","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"champignon","variant":"légume","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"poivron","variant":"légume","type":"HYP","locale":"fr_CA"}
{"index":{}}
{"term":"le","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"la","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"les","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"un","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"une","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"des","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"du","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"de","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"et","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"ou","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"avec","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"pour","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"dans","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"sur","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"au","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"aux","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"à","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"en","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"très","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"bien","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"bon","variant":"","type":"SW","locale":"fr_CA"}
{"index":{}}
{"term":"bonne","variant":"","type":"SW","locale":"fr_CA"}
NDJSON

bulk_index "linguistic_fr_ca" "${TMPDIR}/linguistic_fr_ca.ndjson"

# ============================================================================
# 8. CONCEPTS — DE market, de_DE locale
#    Sources: hellofresh.de/menus, hellofresh.de/recipes
#    German food taxonomy — proteins, vegetables, grains, cuisines, dietary, cooking methods
# ============================================================================
echo "==> Indexing DE (de_DE) concepts..."

cat > "${TMPDIR}/concepts_de_de.ndjson" << 'NDJSON'
{"index":{"_id":"de-cat-haehnchen"}}
{"id":"de-cat-haehnchen","label":"hähnchen","field":"category","aliases":["huhn","hühnchen","geflügel","hähnchenbrust","hähnchengeschnetzeltes","hähnchenkeule","bio-hähnchen"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-haehnchenbrustfilet"}}
{"id":"de-cat-haehnchenbrustfilet","label":"hähnchenbrust","field":"category","aliases":["hähnchenbrustfilet","hühnerbrustfilet","brustfilet"],"weight":12,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-haehnchenkeule"}}
{"id":"de-cat-haehnchenkeule","label":"hähnchenkeule","field":"category","aliases":["hähnchensckenkel","keule"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-rindfleisch"}}
{"id":"de-cat-rindfleisch","label":"rindfleisch","field":"category","aliases":["rind","beef"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-rinderhack"}}
{"id":"de-cat-rinderhack","label":"rinderhackfleisch","field":"category","aliases":["rinderhack","hackfleisch","bio-rinderhack","weiderinderhackfleisch","gehacktes"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-rindersteak"}}
{"id":"de-cat-rindersteak","label":"rindersteak","field":"category","aliases":["steak","bio-rindersteak","hüftsteak","hüftsteak vom weiderind","rindersteakstreifen"],"weight":12,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-rinderhackbaellchen"}}
{"id":"de-cat-rinderhackbaellchen","label":"rinderhackbällchen","field":"category","aliases":["hackbällchen","fleischbällchen","frikadellen","bouletten"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-schweinefleisch"}}
{"id":"de-cat-schweinefleisch","label":"schweinefleisch","field":"category","aliases":["schwein","bio-schweinefleisch"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-schweinemedaillons"}}
{"id":"de-cat-schweinemedaillons","label":"schweinemedaillons","field":"category","aliases":["schweinefilet","schweinefiletmedaillons"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-schweineschnitzel"}}
{"id":"de-cat-schweineschnitzel","label":"schweineschnitzel","field":"category","aliases":["schnitzel","wiener schnitzel","paniertes schnitzel"],"weight":12,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-schweinelachssteak"}}
{"id":"de-cat-schweinelachssteak","label":"schweinelachssteak","field":"category","aliases":["schweinesteak","lachssteak vom schwein"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-bacon"}}
{"id":"de-cat-bacon","label":"bacon","field":"category","aliases":["speck","frühstücksspeck","räucherspeck"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-wurst"}}
{"id":"de-cat-wurst","label":"wurst","field":"category","aliases":["würstchen","wiener würstchen","bratwurst","thüringer","currywurst"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-schinken"}}
{"id":"de-cat-schinken","label":"schinken","field":"category","aliases":["krustenschinken","kochschinken","räucherschinken"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-hirsch"}}
{"id":"de-cat-hirsch","label":"hirsch","field":"category","aliases":["hirschsteak","wild","wildfleisch","reh"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-lachs"}}
{"id":"de-cat-lachs","label":"lachs","field":"category","aliases":["lachsfilet","norwegisches lachsfilet","räucherlachs","bio-lachs"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-thunfisch"}}
{"id":"de-cat-thunfisch","label":"thunfisch","field":"category","aliases":["thunfischsteak","thunfisch-bouletten"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-seehecht"}}
{"id":"de-cat-seehecht","label":"seehecht","field":"category","aliases":["hecht","weißfisch"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-seelachs"}}
{"id":"de-cat-seelachs","label":"seelachs","field":"category","aliases":["seelachsfilet","fischfilet","backfisch"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-buntbarsch"}}
{"id":"de-cat-buntbarsch","label":"buntbarsch","field":"category","aliases":["barsch","tilapia"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-garnelen"}}
{"id":"de-cat-garnelen","label":"garnelen","field":"category","aliases":["shrimps","krabben","riesengarnelen"],"weight":11,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-tofu"}}
{"id":"de-cat-tofu","label":"tofu","field":"category","aliases":["geräucherter tofu","chili-tofu","seidentofu","räuchertofu"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-halloumi"}}
{"id":"de-cat-halloumi","label":"halloumi","field":"category","aliases":["grillkäse"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kartoffel"}}
{"id":"de-cat-kartoffel","label":"kartoffel","field":"ingredient","aliases":["kartoffeln","drillinge","ofenkartoffeln","salzkartoffeln","pellkartoffeln","kartoffelwedges","kartoffelspalten"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kartoffelstampf"}}
{"id":"de-cat-kartoffelstampf","label":"kartoffelstampf","field":"ingredient","aliases":["kartoffelpüree","stampf","kartoffelbrei"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kartoffelpuffer"}}
{"id":"de-cat-kartoffelpuffer","label":"kartoffelpuffer","field":"ingredient","aliases":["reibekuchen","kartoffelpfannkuchen"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-suesskartoffel"}}
{"id":"de-cat-suesskartoffel","label":"süßkartoffel","field":"ingredient","aliases":["süsskartoffel","süßkartoffeln","batate"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-brokkoli"}}
{"id":"de-cat-brokkoli","label":"brokkoli","field":"ingredient","aliases":["broccoli","bimi","bimi-brokkolini"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-blumenkohl"}}
{"id":"de-cat-blumenkohl","label":"blumenkohl","field":"ingredient","aliases":["karfiol"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-spinat"}}
{"id":"de-cat-spinat","label":"spinat","field":"ingredient","aliases":["babyspinat","blattspinat"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-paprika"}}
{"id":"de-cat-paprika","label":"paprika","field":"ingredient","aliases":["paprikaschote","spitzpaprika","rote paprika","gelbe paprika"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-champignons"}}
{"id":"de-cat-champignons","label":"champignons","field":"ingredient","aliases":["pilze","kräuterseitlinge","portobello","portobello-champignon"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-zucchini"}}
{"id":"de-cat-zucchini","label":"zucchini","field":"ingredient","aliases":["zucchinis"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-karotte"}}
{"id":"de-cat-karotte","label":"karotte","field":"ingredient","aliases":["karotten","möhre","möhren","rüebli"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-mais"}}
{"id":"de-cat-mais","label":"mais","field":"ingredient","aliases":["maiskolben","zuckermais"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-lauch"}}
{"id":"de-cat-lauch","label":"lauch","field":"ingredient","aliases":["porree","frühlingszwiebeln","frühlingszwiebel"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-spitzkohl"}}
{"id":"de-cat-spitzkohl","label":"spitzkohl","field":"ingredient","aliases":["kohl","weißkohl","rotkohl"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-pak-choi"}}
{"id":"de-cat-pak-choi","label":"pak choi","field":"ingredient","aliases":["bok choy","chinakohl"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-aubergine"}}
{"id":"de-cat-aubergine","label":"aubergine","field":"ingredient","aliases":["auberginen","melanzane"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-bohnen"}}
{"id":"de-cat-bohnen","label":"bohnen","field":"ingredient","aliases":["grüne bohnen","kidneybohnen","schwarze bohnen","cannellinibohnen","butterbohnen"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-zuckerschoten"}}
{"id":"de-cat-zuckerschoten","label":"zuckerschoten","field":"ingredient","aliases":["zuckererbsen","mangetout"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kohlrabi"}}
{"id":"de-cat-kohlrabi","label":"kohlrabi","field":"ingredient","aliases":["kohlrübe"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-fenchel"}}
{"id":"de-cat-fenchel","label":"fenchel","field":"ingredient","aliases":["fenchelknolle"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kuerbis"}}
{"id":"de-cat-kuerbis","label":"kürbis","field":"ingredient","aliases":["kurbis","hokkaido","butternut","muskatkürbis"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-avocado"}}
{"id":"de-cat-avocado","label":"avocado","field":"ingredient","aliases":["guacamole"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kichererbsen"}}
{"id":"de-cat-kichererbsen","label":"kichererbsen","field":"ingredient","aliases":["kichererbse"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-linsen"}}
{"id":"de-cat-linsen","label":"linsen","field":"ingredient","aliases":["rote linsen","berglinsen","belugalinsen"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-reis"}}
{"id":"de-cat-reis","label":"reis","field":"ingredient","aliases":["jasminreis","basmatireis","wildreis","langkornreis","limettenreis","kokosreis"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-pasta"}}
{"id":"de-cat-pasta","label":"pasta","field":"ingredient","aliases":["nudeln","penne","fusilli","linguine","fettuccine","conchiglie","strozzapreti","spaghetti","rigatoni"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-spaetzle"}}
{"id":"de-cat-spaetzle","label":"spätzle","field":"ingredient","aliases":["eierspätzle","käsespätzle","schwäbische spätzle"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-gnocchi"}}
{"id":"de-cat-gnocchi","label":"gnocchi","field":"ingredient","aliases":["kartoffelgnocchi"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-couscous"}}
{"id":"de-cat-couscous","label":"couscous","field":"ingredient","aliases":["perlcouscous"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-bulgur"}}
{"id":"de-cat-bulgur","label":"bulgur","field":"ingredient","aliases":["bulgurweizen"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-fladenbrot"}}
{"id":"de-cat-fladenbrot","label":"fladenbrot","field":"ingredient","aliases":["naan","pita","pitabrot","tortilla","wrap"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-maultaschen"}}
{"id":"de-cat-maultaschen","label":"maultaschen","field":"ingredient","aliases":["schwäbische maultaschen"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-knoedel"}}
{"id":"de-cat-knoedel","label":"knödel","field":"ingredient","aliases":["klöße","semmelknödel","kartoffelknödel","mini-knödel"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-kaese"}}
{"id":"de-cat-kaese","label":"käse","field":"ingredient","aliases":["feta","mozzarella","gouda","cheddar","parmesan","halloumi","hirtenkäse","ziegenfrischkäse","camembert","raclette-käse","burrata","ricotta"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-erdnuesse"}}
{"id":"de-cat-erdnuesse","label":"erdnüsse","field":"ingredient","aliases":["erdnuss","cashew","cashews","haselnüsse","mandeln","nüsse","kürbiskerne","sonnenblumenkerne"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-baerlauch"}}
{"id":"de-cat-baerlauch","label":"bärlauch","field":"ingredient","aliases":["bärlauchschmand","bärlauchpesto","bärlauch-butter"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-deutsch"}}
{"id":"de-cuisine-deutsch","label":"deutsch","field":"cuisine","aliases":["deutsche küche","traditionell","hausmannskost","klassiker"],"weight":10,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-italienisch"}}
{"id":"de-cuisine-italienisch","label":"italienisch","field":"cuisine","aliases":["italienische küche","italia","italian"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-mexikanisch"}}
{"id":"de-cuisine-mexikanisch","label":"mexikanisch","field":"cuisine","aliases":["mexicanisch","tex-mex","texmex","lateinamerikanisch"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-griechisch"}}
{"id":"de-cuisine-griechisch","label":"griechisch","field":"cuisine","aliases":["greek","griechenland","gyros","souvlaki"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-asiatisch"}}
{"id":"de-cuisine-asiatisch","label":"asiatisch","field":"cuisine","aliases":["asiatische küche","asia"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-japanisch"}}
{"id":"de-cuisine-japanisch","label":"japanisch","field":"cuisine","aliases":["japan","sushi","ramen","udon","donburi"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-koreanisch"}}
{"id":"de-cuisine-koreanisch","label":"koreanisch","field":"cuisine","aliases":["korea","bibimbap","bulgogi","kimchi"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-thailaendisch"}}
{"id":"de-cuisine-thailaendisch","label":"thailändisch","field":"cuisine","aliases":["thai","thailand","pad thai","curry"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-chinesisch"}}
{"id":"de-cuisine-chinesisch","label":"chinesisch","field":"cuisine","aliases":["china","kantonesisch","szechuan","kung pao"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-vietnamesisch"}}
{"id":"de-cuisine-vietnamesisch","label":"vietnamesisch","field":"cuisine","aliases":["vietnam","pho"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-indisch"}}
{"id":"de-cuisine-indisch","label":"indisch","field":"cuisine","aliases":["indien","curry","masala","tikka","korma","dal"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-mediterran"}}
{"id":"de-cuisine-mediterran","label":"mediterran","field":"cuisine","aliases":["mittelmeer","mittelmeerküche"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-tuerkisch"}}
{"id":"de-cuisine-tuerkisch","label":"türkisch","field":"cuisine","aliases":["türkei","döner","kebab","lahmacun"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-orientalisch"}}
{"id":"de-cuisine-orientalisch","label":"orientalisch","field":"cuisine","aliases":["naher osten","arabisch","libanesisch","persisch","schawarma","falafel"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-franzoesisch"}}
{"id":"de-cuisine-franzoesisch","label":"französisch","field":"cuisine","aliases":["frankreich","bistro","provenzalisch"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-spanisch"}}
{"id":"de-cuisine-spanisch","label":"spanisch","field":"cuisine","aliases":["spanien","tapas"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-karibisch"}}
{"id":"de-cuisine-karibisch","label":"karibisch","field":"cuisine","aliases":["karibik","jamaikanisch","jerk"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-marokkanisch"}}
{"id":"de-cuisine-marokkanisch","label":"marokkanisch","field":"cuisine","aliases":["marokko","nordafrikanisch","tagine"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-indonesisch"}}
{"id":"de-cuisine-indonesisch","label":"indonesisch","field":"cuisine","aliases":["indonesien","nasi goreng","sambal"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-skandinavisch"}}
{"id":"de-cuisine-skandinavisch","label":"skandinavisch","field":"cuisine","aliases":["nordisch","schwedisch","dänisch"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cuisine-amerikanisch"}}
{"id":"de-cuisine-amerikanisch","label":"amerikanisch","field":"cuisine","aliases":["american","usa","burger","bbq"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-vegetarisch"}}
{"id":"de-diet-vegetarisch","label":"vegetarisch","field":"dietary","aliases":["veggie","fleischlos","ohne fleisch"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-vegan"}}
{"id":"de-diet-vegan","label":"vegan","field":"dietary","aliases":["pflanzlich","rein pflanzlich"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-low-carb"}}
{"id":"de-diet-low-carb","label":"low carb","field":"dietary","aliases":["wenig kohlenhydrate","kohlenhydratarm","kalorienreduziert"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-proteinreich"}}
{"id":"de-diet-proteinreich","label":"proteinreich","field":"dietary","aliases":["high protein","eiweißreich","viel protein"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-glutenfrei"}}
{"id":"de-diet-glutenfrei","label":"glutenfrei","field":"dietary","aliases":["gluten-free","ohne gluten","zöliakiefreundlich"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-laktosefrei"}}
{"id":"de-diet-laktosefrei","label":"laktosefrei","field":"dietary","aliases":["ohne laktose","milchfrei"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-ballaststoffreich"}}
{"id":"de-diet-ballaststoffreich","label":"ballaststoffreich","field":"dietary","aliases":["viel ballaststoffe","faserreich"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-kalorienarm"}}
{"id":"de-diet-kalorienarm","label":"kalorienarm","field":"dietary","aliases":["kalorien im blick","wenig kalorien","leicht","light"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-familienfreundlich"}}
{"id":"de-diet-familienfreundlich","label":"familienfreundlich","field":"dietary","aliases":["family","für die ganze familie","kinderfreundlich"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-diet-viel-gemuese"}}
{"id":"de-diet-viel-gemuese","label":"viel gemüse","field":"dietary","aliases":["gemüsereich","extra gemüse"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-burger"}}
{"id":"de-meal-burger","label":"burger","field":"meal_type","aliases":["hamburger","cheeseburger"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-tacos"}}
{"id":"de-meal-tacos","label":"tacos","field":"meal_type","aliases":["taco","quesadilla","quesadillas","burrito","enchiladas"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-bowl"}}
{"id":"de-meal-bowl","label":"bowl","field":"meal_type","aliases":["bowls","reisbowl","buddha bowl"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-pfanne"}}
{"id":"de-meal-pfanne","label":"pfanne","field":"meal_type","aliases":["pfannengericht","bratpfanne"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-suppe"}}
{"id":"de-meal-suppe","label":"suppe","field":"meal_type","aliases":["suppen","eintopf","gulasch"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-salat"}}
{"id":"de-meal-salat","label":"salat","field":"meal_type","aliases":["salate","salatbowl","beilagensalat"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-auflauf"}}
{"id":"de-meal-auflauf","label":"auflauf","field":"meal_type","aliases":["gratin","überbacken","gratiniert"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-flammkuchen"}}
{"id":"de-meal-flammkuchen","label":"flammkuchen","field":"meal_type","aliases":["tarte flambée","elsässer flammkuchen"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-sandwich"}}
{"id":"de-meal-sandwich","label":"sandwich","field":"meal_type","aliases":["sandwichs","brot","belegtes brot"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-pizza"}}
{"id":"de-meal-pizza","label":"pizza","field":"meal_type","aliases":["flatbread","fladen"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-risotto"}}
{"id":"de-meal-risotto","label":"risotto","field":"meal_type","aliases":["reisotto"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-meal-curry"}}
{"id":"de-meal-curry","label":"curry","field":"meal_type","aliases":["currygericht","erdnuss-curry","kokos-curry"],"weight":9,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-schnell"}}
{"id":"de-method-schnell","label":"schnell","field":"cooking_method","aliases":["extra schnell","15 minuten","20 minuten","fix","flott","blitzrezept"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-one-pot"}}
{"id":"de-method-one-pot","label":"one pot","field":"cooking_method","aliases":["one-pot","eintopf","alles in einem topf"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-one-pan"}}
{"id":"de-method-one-pan","label":"one pan","field":"cooking_method","aliases":["one-pan","eine pfanne"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-ofen"}}
{"id":"de-method-ofen","label":"ofen","field":"cooking_method","aliases":["aus dem ofen","ofengericht","überbacken","vom blech"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-pfanne"}}
{"id":"de-method-pfanne","label":"pfanne","field":"cooking_method","aliases":["angebraten","gebraten","in der pfanne"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-gegrillt"}}
{"id":"de-method-gegrillt","label":"gegrillt","field":"cooking_method","aliases":["grillen","grill","bbq","barbecue"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-air-fryer"}}
{"id":"de-method-air-fryer","label":"air fryer","field":"cooking_method","aliases":["heißluftfritteuse","airfryer"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-cremig"}}
{"id":"de-method-cremig","label":"cremig","field":"cooking_method","aliases":["sahnig","in sahnesoße"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-knusprig"}}
{"id":"de-method-knusprig","label":"knusprig","field":"cooking_method","aliases":["kross","crispy","paniert","knusperbrösel"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-method-premium"}}
{"id":"de-method-premium","label":"premium","field":"cooking_method","aliases":["gourmet","besonderer anlass","date night"],"weight":8,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-teriyaki"}}
{"id":"de-cat-teriyaki","label":"teriyaki","field":"ingredient","aliases":["teriyaki-soße","teriyaki sauce"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-harissa"}}
{"id":"de-cat-harissa","label":"harissa","field":"ingredient","aliases":["harissa-paste","scharfe paste"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-sambal"}}
{"id":"de-cat-sambal","label":"sambal","field":"ingredient","aliases":["sambal oelek","chili-paste"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-hoisin"}}
{"id":"de-cat-hoisin","label":"hoisin","field":"ingredient","aliases":["hoisin-soße","hoisinsauce"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-senfsosse"}}
{"id":"de-cat-senfsosse","label":"senf","field":"ingredient","aliases":["senfsoße","senfbutter","honig-senf","dijon-senf"],"weight":7,"locale":"de_DE","market":"DE"}
{"index":{"_id":"de-cat-joghurt"}}
{"id":"de-cat-joghurt","label":"joghurt","field":"ingredient","aliases":["joghurtdip","kräuterjoghurt","zaziki","tzatziki"],"weight":7,"locale":"de_DE","market":"DE"}
NDJSON

bulk_index "concepts_de_de" "${TMPDIR}/concepts_de_de.ndjson"

# ============================================================================
# 8b. LINGUISTIC — DE market, de_DE locale
#     German synonyms, hypernyms, stop words
# ============================================================================
echo "==> Indexing DE (de_DE) linguistic entries..."

cat > "${TMPDIR}/linguistic_de_de.ndjson" << 'NDJSON'
{"index":{}}
{"term":"hähnchen","variant":"huhn","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"huhn","variant":"hähnchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"hähnchen","variant":"geflügel","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"geflügel","variant":"hähnchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"rindfleisch","variant":"rind","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"rind","variant":"rindfleisch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"schweinefleisch","variant":"schwein","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"schwein","variant":"schweinefleisch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"hackfleisch","variant":"gehacktes","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"gehacktes","variant":"hackfleisch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"hackfleisch","variant":"rinderhack","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"rinderhack","variant":"hackfleisch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"garnelen","variant":"shrimps","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"shrimps","variant":"garnelen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"garnelen","variant":"krabben","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"krabben","variant":"garnelen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"kartoffel","variant":"kartoffeln","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"kartoffelpüree","variant":"kartoffelstampf","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"kartoffelstampf","variant":"kartoffelpüree","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"kartoffelpuffer","variant":"reibekuchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"reibekuchen","variant":"kartoffelpuffer","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"nudeln","variant":"pasta","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"pasta","variant":"nudeln","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"pilze","variant":"champignons","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"champignons","variant":"pilze","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"möhre","variant":"karotte","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"karotte","variant":"möhre","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"möhren","variant":"karotten","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"karotten","variant":"möhren","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"lauch","variant":"porree","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"porree","variant":"lauch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"frühlingszwiebeln","variant":"lauchzwiebeln","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"lauchzwiebeln","variant":"frühlingszwiebeln","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"frikadellen","variant":"bouletten","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"bouletten","variant":"frikadellen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"frikadellen","variant":"hackbällchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"hackbällchen","variant":"frikadellen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"fleischbällchen","variant":"hackbällchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"brötchen","variant":"semmel","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"semmel","variant":"brötchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"sahne","variant":"rahm","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"rahm","variant":"sahne","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"quark","variant":"topfen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"topfen","variant":"quark","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"blumenkohl","variant":"karfiol","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"karfiol","variant":"blumenkohl","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"tomate","variant":"paradeiser","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"paradeiser","variant":"tomate","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"knödel","variant":"klöße","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"klöße","variant":"knödel","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"pfannkuchen","variant":"palatschinken","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"palatschinken","variant":"pfannkuchen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"vegetarisch","variant":"veggie","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"veggie","variant":"vegetarisch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"fleischlos","variant":"vegetarisch","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"schnell","variant":"fix","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"fix","variant":"schnell","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"einfach","variant":"leicht","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"leicht","variant":"einfach","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"würzig","variant":"pikant","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"pikant","variant":"würzig","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"scharf","variant":"spicy","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"spicy","variant":"scharf","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"zaziki","variant":"tzatziki","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"tzatziki","variant":"zaziki","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"bbq","variant":"grillen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"grillen","variant":"bbq","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"barbecue","variant":"grillen","type":"SYN","locale":"de_DE"}
{"index":{}}
{"term":"hähnchen","variant":"fleisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"rindfleisch","variant":"fleisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"schweinefleisch","variant":"fleisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"lachs","variant":"fisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"thunfisch","variant":"fisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"seehecht","variant":"fisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"seelachs","variant":"fisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"buntbarsch","variant":"fisch","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"garnelen","variant":"meeresfrüchte","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"spaghetti","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"penne","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"linguine","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"fusilli","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"fettuccine","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"rigatoni","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"conchiglie","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"gnocchi","variant":"pasta","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"jasminreis","variant":"reis","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"basmatireis","variant":"reis","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"brokkoli","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"spinat","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"zucchini","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"blumenkohl","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"paprika","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"karotte","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"champignons","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"kohlrabi","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"kürbis","variant":"gemüse","type":"HYP","locale":"de_DE"}
{"index":{}}
{"term":"der","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"die","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"das","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"ein","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"eine","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"und","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"oder","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"mit","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"für","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"von","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"in","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"auf","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"an","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"zu","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"nach","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"aus","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"bei","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"vom","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"zum","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"zur","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"sehr","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"gut","variant":"","type":"SW","locale":"de_DE"}
{"index":{}}
{"term":"lecker","variant":"","type":"SW","locale":"de_DE"}
NDJSON

bulk_index "linguistic_de_de" "${TMPDIR}/linguistic_de_de.ndjson"

# ============================================================================
# FILE-BASED LOCALES — Load from locale-data/ directory
# ============================================================================
if [ -d "${LOCALE_DATA_DIR}" ]; then
  for concepts_file in "${LOCALE_DATA_DIR}"/*_concepts.ndjson; do
    [ -f "${concepts_file}" ] || continue
    locale=$(basename "${concepts_file}" _concepts.ndjson)
    # Skip locales already indexed inline above
    case "${locale}" in en_gb|en_us|en_ca|fr_ca|de_de) continue ;; esac
    echo "==> Indexing ${locale} concepts (from file)..."
    bulk_index "concepts_${locale}" "${concepts_file}"
  done
  for linguistic_file in "${LOCALE_DATA_DIR}"/*_linguistic.ndjson; do
    [ -f "${linguistic_file}" ] || continue
    locale=$(basename "${linguistic_file}" _linguistic.ndjson)
    case "${locale}" in en_gb|en_us|en_ca|fr_ca|de_de) continue ;; esac
    echo "==> Indexing ${locale} linguistic (from file)..."
    bulk_index "linguistic_${locale}" "${linguistic_file}"
  done
fi

# ---------------------------------------------------------------------------
# Refresh all indices
# ---------------------------------------------------------------------------
echo "==> Refreshing indices..."
for locale in ${ALL_LOCALES}; do
  curl -s -X POST "${OS_URL}/concepts_${locale}/_refresh" > /dev/null
  curl -s -X POST "${OS_URL}/linguistic_${locale}/_refresh" > /dev/null
done
echo "  All indices refreshed."

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "==> Done! Index counts:"
for locale in ${ALL_LOCALES}; do
  concepts=$(curl -s "${OS_URL}/concepts_${locale}/_count" | jq -r '.count // 0')
  linguistic=$(curl -s "${OS_URL}/linguistic_${locale}/_count" | jq -r '.count // 0')
  printf "  %-20s concepts=%-4s linguistic=%-4s\n" "${locale}" "${concepts}" "${linguistic}"
done
echo ""
echo "==> Sample queries to try:"
echo ""
echo "  # GB market"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"veggie thai green curry","locale":"en_GB","market":"GB"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"quick chicken stir fry","locale":"en_GB","market":"GB"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"spag bol","locale":"en_GB","market":"GB"}'\'' | jq'
echo ""
echo "  # US market"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"chicken parm with fettuccine alfredo","locale":"en_US","market":"US"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"gluten free steak and potatoes","locale":"en_US","market":"US"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"easy shrimp tacos with chipotle crema","locale":"en_US","market":"US"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"quick korean bibimbap bowl","locale":"en_US","market":"US"}'\'' | jq'
echo ""
echo "  # CA market (English)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"quick chicken stir fry with bok choy","locale":"en_CA","market":"CA"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"poutine with peameal bacon","locale":"en_CA","market":"CA"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"gluten free salmon bowl","locale":"en_CA","market":"CA"}'\'' | jq'
echo ""
echo "  # CA market (French)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"poulet grillé avec riz jasmin","locale":"fr_CA","market":"CA"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"pâtes au saumon rapide","locale":"fr_CA","market":"CA"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"tourtière québécoise","locale":"fr_CA","market":"CA"}'\'' | jq'
echo ""
echo "  # DE market (German)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"schnelles hähnchen curry mit reis","locale":"de_DE","market":"DE"}'\'' | jq'
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"schnitzel mit kartoffelstampf","locale":"de_DE","market":"DE"}'\'' | jq'
echo ""
echo "  # AU market"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"quick chicken stir fry with capsicum","locale":"en_AU","market":"AU"}'\'' | jq'
echo ""
echo "  # NL market (Dutch)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"stamppot met gehaktballetjes","locale":"nl_NL","market":"NL"}'\'' | jq'
echo ""
echo "  # IT market (Italian)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"pasta alla carbonara con pancetta","locale":"it_IT","market":"IT"}'\'' | jq'
echo ""
echo "  # SE market (Swedish)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"köttbullar med potatis","locale":"sv_SE","market":"SE"}'\'' | jq'
echo ""
echo "  # JP market (Japanese)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"鶏肉のカレー丼","locale":"ja_JP","market":"JP"}'\'' | jq'
echo ""
echo "  # ES market (Spanish)"
echo '  curl -s -X POST "http://localhost:8080/v1/analyze?debug=true" -H "Content-Type: application/json" -d '\''{"query":"pollo al horno con patatas","locale":"es_ES","market":"ES"}'\'' | jq'
