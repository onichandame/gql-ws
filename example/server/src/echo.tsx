import { FC, useCallback, useState } from "react";

import { useClient } from "./client";

export const Echo: FC = () => {
  const client = useClient();
  const [echoInput, setEchoInput] = useState(``);
  const [echo, setEcho] = useState(``);
  const sendEcho = useCallback(
    async (input: string) =>
      client?.subscribe<{ echo: string }>(
        {
          query: `query($input:String!){echo(input:$input)}`,
          variables: { input },
        },
        {
          next: (next) => {
            console.log(next);
            if (next.data) setEcho(next.data.echo);
            if (next.errors) console.error(next.errors);
          },
          error: () => {},
          complete: () => {},
        }
      ),
    [client]
  );
  return (
    <div>
      <input
        type="text"
        onChange={(e) => {
          setEchoInput(e.currentTarget.value);
        }}
      />
      <button
        type="button"
        onClick={() => {
          sendEcho(echoInput);
        }}
      >
        echo: {echo}
      </button>
    </div>
  );
};
