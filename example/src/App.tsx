import { useState } from "react";
import { useClient } from "./client";

import logo from "./logo.svg";
import "./App.css";

function App() {
  const client = useClient();
  const [echo, setEcho] = useState(``);
  const sendEcho = async () => {
    client?.subscribe<{ echo: string }>(
      { query: `query{echo}` },
      {
        next: (next) => {
          console.log(next);
          if (next.data) setEcho(next.data.echo);
          if (next.errors) console.error(next.errors);
        },
        error: () => {},
        complete: () => {},
      }
    );
  };
  return (
    <div className="App">
      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
        <p>Hello Vite + React!</p>
        <p>
          <button type="button" onClick={sendEcho}>
            echo: {echo}
          </button>
        </p>
        <p>
          Edit <code>App.tsx</code> and save to test HMR updates.
        </p>
        <p>
          <a
            className="App-link"
            href="https://reactjs.org"
            target="_blank"
            rel="noopener noreferrer"
          >
            Learn React
          </a>
          {" | "}
          <a
            className="App-link"
            href="https://vitejs.dev/guide/features.html"
            target="_blank"
            rel="noopener noreferrer"
          >
            Vite Docs
          </a>
        </p>
      </header>
    </div>
  );
}

export default App;
