import re
import requests  # 发送请求
from bs4 import BeautifulSoup  # 解析网页
import pandas as pd  # 存取csv
from time import sleep  # 等待时间
import json
from dataclasses import dataclass, asdict
from multiprocessing import Process


class Utils:
    def trims(str: str) -> str:
        return str.replace('\n', '').replace('\r', '').replace(' ', '')


@dataclass
class Book:
    ISBN: str
    name: str
    author: str
    publishHouse: str
    price: str
    described: str
    imgUrl: str


@dataclass
class BookBuilder:
    bookISBN = ""
    bookName: str = ""
    bookImgUrl: str = ""
    bookStar: int = 0
    bookStarPeople: int = 0
    bookComment: str = ""
    bookCountry: str = ""
    bookAuthor: str = ""
    bookTranslator: str = ""
    bookPublisher: str = ""
    bookYear: str = ""
    bookPrice: str = ""
    described: str = ""

    def build(self) -> Book:
        return Book(Utils.trims(self.bookISBN), Utils.trims(self.bookName), Utils.trims(self.bookAuthor), Utils.trims(self.bookPublisher), Utils.trims(self.bookPrice), Utils.trims(self.described), Utils.trims(self.bookImgUrl))


def get_book_info(url: str, headers: dict) -> list[BookBuilder]:
    res = requests.get(url, headers=headers)
    soup = BeautifulSoup(res.text, 'html.parser')
    book_builder_list: list[BookBuilder] = []
    for bookElement in soup.select('table'):
        book_builder = BookBuilder()
        book_builder.bookName = bookElement.find('div', class_='pl2').a.text.strip().replace(
            '"', '')

        book_builder.bookImgUrl = bookElement.find(
            'img').attrs['src']

        book_builder.bookStar = bookElement.find('div', class_='star').find(
            'span', class_='rating_nums').text

        star_people_element = bookElement.find(
            'div', class_='star').find('span', class_='pl')
        star_people_text = star_people_element.text.strip()
        book_builder.bookStarPeople = re.search(
            r'\d+', star_people_text).group() if re.search(r'\d+', star_people_text) else ""
        try:
            book_builder.bookComment = bookElement.find(
                'p', class_='quote').span.text.strip()  # 评论
        except:
            pass

        book_info_text = bookElement.find('p', class_='pl').text.strip()

        pattern = re.compile(
            r'(?:\[(.*?)\] )?(.*?)(?: \/ (.*?))?(?: \/ (.*?)) \/ (.*?) \/ (.*?)元')
        matches = pattern.match(book_info_text)

        book_builder.bookCountry = matches.group(
            1) if matches and matches.group(1) else ""

        raw_author = matches.group(2) if matches and matches.group(2) else ""
        book_builder.bookAuthor = raw_author.replace(
            ' 著', '') if raw_author else ""

        book_builder.bookTranslator = matches.group(
            3) if matches and matches.group(3) else ""
        book_builder.bookPublisher = matches.group(
            4) if matches and matches.group(4) else ""
        book_builder.bookYear = matches.group(
            5) if matches and matches.group(5) else ""
        book_builder.bookPrice = matches.group(
            6) if matches and matches.group(6) else ""

        more_info_url = bookElement.find(
            'div', class_="pl2").find("a").attrs['href']
        more_info_res = requests.get(more_info_url, headers=headers)
        infoDocument = BeautifulSoup(more_info_res.text, 'html.parser')
        infoElement = infoDocument.find('div', id='info')
        book_builder.bookISBN = infoElement.text.split("ISBN:")[1].strip(
        ) if infoElement.text.count("ISBN") else infoElement.text.split("统一书号:")[1].strip()
        book_builder.described = infoDocument.find(
            'div', class_='related_info').find_next('p').text.strip()
        book_builder_list.append(book_builder)
    return book_builder_list


def send_book_to_db(books: list[BookBuilder]):
    baseUrl = "http://localhost:3000/stock/addbook"
    for book in books:
        res = requests.post(baseUrl, asdict(book.build()))
        print(res.text)


def get_book_by_url_and_header(url: str, idx: int, header: dict):
    print('开始爬取第{}页，地址是:{}'.format(str(idx + 1), url))
    books = get_book_info(url, header)
    send_book_to_db(books)


def async_get_book(url: str, idx: int, header: dict) -> Process:
    return Process(target=get_book_by_url_and_header, args=(url, idx, header))


def await_threads(threads: list[Process]):
    for thread in threads:
        sleep(2)
        thread.run()
    for thread in threads:
        thread.join()


def main():
    headers = {
        'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36'}
    thread_list = []
    for i in range(10):  # 爬取共10页，每页25条数据
        page_url = 'https://book.douban.com/top250?start={}'.format(
            str(i * 25))
        thread = async_get_book(page_url, i, headers)
        thread_list.append(thread)
    await_threads(thread_list)


if __name__ == "__main__":
    main()
