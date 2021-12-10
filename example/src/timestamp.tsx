import { FC, useCallback, useReducer, useState } from "react";
import { useClient } from "./client";

export const Timestamp: FC = () => {
  const [ts, setTS] = useState<null | Date>(null);
  const [started, toggleStarted] = useReducer((v) => !v, false);
  const [stop, setStop] = useState(() => () => {});
  const client = useClient();
  const startTS = useCallback(async () => {
    const unsubscribe = client?.subscribe<{ timestamp: Date }>(
      { query: `subscription{timestamp}` },
      {
        complete: () => {
          console.log(`completed`);
        },
        error: (e) => {
          console.error(e);
        },
        next: (next) => {
          console.log(`next: ${next.data?.timestamp}`);
          if (next.data) {
            setTS(next.data.timestamp);
          }
        },
      }
    );
    unsubscribe && setStop(() => unsubscribe);
  }, [client]);
  return (
    <div>
      <button
        onClick={() => {
          if (started) {
            stop && stop();
          } else {
            startTS();
          }
          toggleStarted();
        }}
      >
        Timestamp: {ts?.toString()}
      </button>
    </div>
  );
};
