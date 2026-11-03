import { apiClient } from './client'

export interface ContactPage {
  image: string
  link: string
  text: string
  contact_info: string
}

export async function getContactPage(): Promise<ContactPage> {
  return (await apiClient.get<ContactPage>('/user/contact-page')).data
}
