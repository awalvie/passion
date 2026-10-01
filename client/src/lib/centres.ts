import { request } from './api';

// The shape of server/api/centre.go's centreResponse.
export type Centre = { id: string; name: string; sessions: string[] };

export async function loadCentres(): Promise<Centre[]> {
	return (await request<{ centres: Centre[] }>('GET', '/api/v1/centres')).centres;
}

export function saveCentre(c: Centre): Promise<Centre> {
	return request<Centre>('PUT', `/api/v1/centres/${c.id}`, { name: c.name, sessions: c.sessions });
}
