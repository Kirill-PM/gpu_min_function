import { useEffect, useRef, useCallback } from 'react';

export function useWebSocket(url: string, onMessage: (data: any) => void) {
  const wsRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);

  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(() => {
    try {
      wsRef.current = new WebSocket(url);

      wsRef.current.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessageRef.current(data);
        } catch (e) {
          console.error('WebSocket message parse error:', e);
        }
      };

      wsRef.current.onopen = () => {
        console.log('WebSocket connected');
      };

      wsRef.current.onclose = () => {
        console.log('WebSocket closed');
      };

      wsRef.current.onerror = (error) => {
        console.error('WebSocket error:', error);
      };
    } catch (e) {
      console.error('WebSocket creation error:', e);
    }

    return () => {
      wsRef.current?.close();
    };
  }, [url]);

  return wsRef.current;
}
