import { Client, createClient } from "graphql-ws";
import { useEffect, useState } from "react";

export const useClient = () => {
  const [client, setClient] = useState<null | Client>(null);
  useEffect(() => {
    const url = `${
      window.location.protocol.startsWith(`https`) ? `wss` : `ws`
    }://${window.location.host}/${window.location.pathname
      .split(`/`)
      .filter((v) => !!v)
      .slice(0, -1)
      .concat(`graphql`)
      .join(`/`)}`;
    console.log(url);
    const c = createClient({ url });
    c.on(`connected`, () => {
      console.log(`connected`);
      setClient(c);
    });
    c.on(`closed`, () => {
      console.log(`closed`);
    });
    c.on(`connecting`, () => {
      console.log(`connecting`);
    });
    c.on(`opened`, () => {
      console.log(`opened`);
    });
    c.on(`error`, (e) => {
      console.error(e);
    });
  }, []);
  return client;
};
