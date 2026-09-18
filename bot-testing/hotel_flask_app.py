from flask import Flask, render_template_string, request

app = Flask(__name__)

# Demo hotel data. Replace/extend this list with your real dataset.
HOTELS = [
    {"name": "Fariyas Resort", "location": "Arsiwalla Villa, Lonavala", "rating": 4.4, "ratings": 5246, "price": 37195, "old_price": 55333, "taxes": 11354, "distance": "2.9 km from Lonavala Railway Station", "breakfast": True, "stars": 5},
    {"name": "The Machan", "location": "Jambulne, Lonavala", "rating": 4.5, "ratings": 1832, "price": 18499, "old_price": 22999, "taxes": 5620, "distance": "14 km from Lonavala Railway Station", "breakfast": True, "stars": 5},
    {"name": "Rhythm Lonavala", "location": "Bhangarwadi, Lonavala", "rating": 4.3, "ratings": 3210, "price": 12600, "old_price": 15999, "taxes": 3820, "distance": "1.8 km from Lonavala Railway Station", "breakfast": True, "stars": 5},
    {"name": "Avion Holiday Resort", "location": "Tungarli, Lonavala", "rating": 4.1, "ratings": 2145, "price": 4999, "old_price": 6999, "taxes": 1519, "distance": "1.2 km from Lonavala Railway Station", "breakfast": False, "stars": 3},
    {"name": "Aamby Valley City", "location": "Aamby Valley, Lonavala", "rating": 4.2, "ratings": 4088, "price": 8999, "old_price": 11999, "taxes": 2730, "distance": "25 km from Lonavala", "breakfast": True, "stars": 5},
    {"name": "Meritas Picaddle Resort", "location": "Tungarli, Lonavala", "rating": 4.2, "ratings": 2864, "price": 7499, "old_price": 9999, "taxes": 2280, "distance": "1.5 km from Lonavala Railway Station", "breakfast": True, "stars": 4},
    {"name": "Upper Deck Resort", "location": "Tungarli Lake Road, Lonavala", "rating": 4.4, "ratings": 1750, "price": 10999, "old_price": 13999, "taxes": 3344, "distance": "3.7 km from Lonavala Railway Station", "breakfast": True, "stars": 4},
    {"name": "Krushnai Resort", "location": "Old Mumbai-Pune Highway, Lonavala", "rating": 3.9, "ratings": 1320, "price": 3899, "old_price": 4999, "taxes": 1187, "distance": "0.8 km from Lonavala Railway Station", "breakfast": False, "stars": 3},
    {"name": "Hilton Shillim Estate Retreat", "location": "Pawana Nagar, Maval", "rating": 4.6, "ratings": 987, "price": 24999, "old_price": 31999, "taxes": 7600, "distance": "30 km from Lonavala", "breakfast": True, "stars": 5},
    {"name": "Sterling Lonavala", "location": "Tungarli, Lonavala", "rating": 4.0, "ratings": 2671, "price": 6299, "old_price": 8499, "taxes": 1918, "distance": "2.4 km from Lonavala Railway Station", "breakfast": True, "stars": 4},
]

PAGE = r"""
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Hotel Search Demo</title>
<style>
*{box-sizing:border-box} body{margin:0;background:#f5f6f8;font-family:Arial,Helvetica,sans-serif;color:#222}
.header{height:72px;background:#fff;border-bottom:1px solid #ddd;display:flex;align-items:center;padding:0 7%;gap:38px}
.logo{font-size:28px;font-weight:800;color:#263c8f}.logo span{background:#ef3d35;color:white;border-radius:8px;padding:3px 8px;margin-left:3px}
.nav{display:flex;gap:30px;font-size:15px}.nav b{color:#0875d1}
.account{margin-left:auto;font-size:14px}.searchbar{background:#fff;padding:18px 7%;display:flex;gap:6px;border-bottom:1px solid #ddd}
.searchbar input,.searchbar button{height:52px;border:1px solid #ddd;background:#fff;padding:0 18px;font-size:15px}
.searchbar input:first-child{width:35%}.searchbar input{width:18%}.searchbar button{width:15%;background:#218cf3;color:white;border:0;font-weight:bold;border-radius:8px}
.layout{display:grid;grid-template-columns:270px 1fr;gap:28px;max-width:1180px;margin:24px auto}
.sidebar{background:#fff;padding:20px;border-radius:8px;height:max-content}.sidebar h3{margin:0 0 18px}.filter{padding:10px 0;border-bottom:1px solid #eee}.filter label{display:block;margin:12px 0;color:#444}
.main h1{font-size:27px;margin:0 0 20px}.banner{background:linear-gradient(90deg,#d9f7f2,#f3ffff);padding:22px;border-radius:18px;margin-bottom:22px;font-size:17px}
.card{background:#fff;border:1px solid #e4e4e4;border-radius:8px;margin-bottom:16px;overflow:hidden}
.cardbody{display:grid;grid-template-columns:245px 1fr 190px;min-height:190px}
.photo{margin:16px;background:linear-gradient(135deg,#dfe4e8,#f7f7f7);border-radius:6px;display:flex;align-items:center;justify-content:center;color:#9ca3a8;font-size:46px}
.info{padding:20px 5px}.info h2{margin:0 0 8px;font-size:22px}.stars{color:#111;letter-spacing:1px}.location{color:#147bd1;margin:7px 0}.breakfast{color:#159b72;margin-top:15px}.price{border-left:1px solid #eee;padding:25px 18px;text-align:right}.rating{font-weight:bold;color:#0875d1}.rating strong{background:#0875d1;color:#fff;padding:7px;border-radius:5px;margin-left:5px}.old{text-decoration:line-through;color:#999;margin-top:22px}.amount{font-size:24px;font-weight:bold}.tax{color:#777;font-size:13px}.book{margin-top:12px;color:#0875d1;font-weight:bold;font-size:13px}.offer{background:#d2faea;padding:13px 18px;color:#147e6c}.empty{padding:40px;background:white;border-radius:8px;text-align:center}
@media(max-width:850px){.layout{grid-template-columns:1fr;margin:15px}.sidebar{display:none}.header{padding:0 15px}.nav{display:none}.searchbar{padding:12px;flex-wrap:wrap}.searchbar input,.searchbar input:first-child,.searchbar button{width:100%}.cardbody{grid-template-columns:1fr}.photo{height:130px}.price{border-left:0;border-top:1px solid #eee;text-align:left}}
</style>
</head>
<body>
<header class="header">
  <div class="logo">stay<span>booker</span></div>
  <nav class="nav"><span>✈ Flights</span><b>🏨 Hotels</b><span>🏡 Villas</span><span>🎫 Holiday</span><span>🚆 Trains</span><span>More ▾</span></nav>
  <div class="account">♡ Wishlist &nbsp;&nbsp; Login / Create Account</div>
</header>

<form class="searchbar" method="get">
  <input name="q" value="{{ q }}" placeholder="CITY, AREA OR PROPERTY">
  <input name="checkin" placeholder="CHECK-IN" value="09/18/2026">
  <input name="checkout" placeholder="CHECK-OUT" value="09/19/2026">
  <input placeholder="ROOMS & GUESTS" value="1 Room, 2 Guests">
  <button type="submit">SEARCH</button>
</form>

<div class="layout">
<aside class="sidebar">
  <h3>For You</h3>
  <div class="filter">
    <label>☐ Rush Deal <small>(433)</small></label>
    <label>☐ Last Minute Deals <small>(102)</small></label>
    <label>☐ Breakfast Included <small>(672)</small></label>
    <label>☐ 5 Star <small>(102)</small></label>
    <label>☐ 4 Star <small>(104)</small></label>
    <label>☐ 3 Star <small>(495)</small></label>
  </div>
  <div class="filter"><h3>Price Per Night</h3>
    <label>☐ ₹ 0 - ₹ 1000</label>
    <label>☐ ₹ 1000 - ₹ 2500</label>
    <label>☐ ₹ 2500 - ₹ 5000</label>
    <label>☐ ₹ 5000+</label>
  </div>
</aside>

<main class="main">
  <h1>{{ hotels|length }} Properties in Pune District</h1>
  <div class="banner"><b>◉ OneCircle</b> &nbsp; Earn & redeem points on eligible properties for additional savings!</div>

  {% if hotels %}
    {% for hotel in hotels %}
    <article class="card">
      <div class="cardbody">
        <div class="photo">🏨</div>
        <div class="info">
          <h2>{{ hotel.name }} <span class="stars">{{ "★" * hotel.stars }}</span></h2>
          <div class="location">{{ hotel.location }}</div>
          <div>{{ hotel.distance }}</div>
          {% if hotel.breakfast %}<div class="breakfast">✓ Breakfast Included</div>{% endif %}
        </div>
        <div class="price">
          <div class="rating">Excellent <strong>{{ hotel.rating }}</strong></div>
          <div>({{ hotel.ratings }} Ratings)</div>
          <div class="old">₹ {{ "{:,}".format(hotel.old_price) }}</div>
          <div class="amount">₹ {{ "{:,}".format(hotel.price) }}</div>
          <div class="tax">+ ₹ {{ "{:,}".format(hotel.taxes) }} taxes & fees<br>Per Night</div>
          <div class="book">Login to Book Now & Pay Later!</div>
        </div>
      </div>
      <div class="offer">KOTAK Credit Card Offer - Get INR 5683 Off!</div>
    </article>
    {% endfor %}
  {% else %}
    <div class="empty">No hotels found for "{{ q }}".</div>
  {% endif %}
</main>
</div>
</body>
</html>
"""

@app.route("/")
def home():
    q = request.args.get("q", "").strip().lower()
    hotels = HOTELS
    if q:
        hotels = [
            h for h in HOTELS
            if q in h["name"].lower() or q in h["location"].lower()
        ]
    return render_template_string(PAGE, hotels=hotels, q=request.args.get("q", ""))

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
