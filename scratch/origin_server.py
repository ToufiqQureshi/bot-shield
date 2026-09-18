from flask import Flask

app = Flask(__name__)

@app.route("/")
def home():
    return "<h1>Welcome to the Real Site!</h1><p>This is protected data.</p>"

if __name__ == "__main__":
    app.run(port=9000)
