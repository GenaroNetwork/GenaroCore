## Go Genaro

Fork from [go-ethereum](https://github.com/ethereum/go-ethereum/tree/v1.8.3).

## Building the source

Building geth requires both a Go (version 1.10 or later) and a C compiler.
You can install them using your favourite package manager.
Once the dependencies are installed, run

    make go-genaro

or, to build the full suite of utilities:

    make all

## Executables

The go-genaro project comes with several wrappers/executables found in the `cmd` directory.

| Command    | Description |
|:----------:|-------------|
| **`go-genaro`** | Our main Genaro CLI client. It is the entry point into the Genaro network. |
| `GenGenaroGenesis` | Generate a genesis json. |
| `SendSynState` | Send synchronization transaction. |
| `others` | To see go-ethereum. |

## Running a go-genaro blockchain

### Generate genesis json

    GenGenaroGenesis -f account.json

### Init blockchain

    go-genaro --datadir datadir init genesis.json

### Start miner

    go-genaro --identity "test" --mine --etherbase "myaccount" --unlock "myaccount" --rpc --rpcaddr "127.0.0.1" --rpcport 8545 --rpccorsdomain "*" --datadir datadir --port "30303" --rpcapi "db,eth,net,web3,personal,admin,miner" console

### Send synchronization transaction

    SendSynState -u "http://127.0.0.1:8545" -t 1 -a "myaccount"

## Develop Dapp

Genaro blockchain support for Ethernet contract.You can directly transplant your Ethernet contract to Genaro network without any changes.
If you need to use the new features of Genaro network, you need to compile the contract with the new [Solc](https://github.com/GenaroNetwork/genaro-solidity).
You can get more development information from the [yellow book](https://github.com/GenaroNetwork/genaro-document).

## Useful links
Our [official network](https://genaro.network/).

## Main Network
To see [mainnet](./mainnet).

## Test Network
To see [testnet](./testnet).

## License

The go-genaro library (i.e. all code outside of the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in our repository in the `COPYING.LESSER` file.

The go-genaro binaries (i.e. all code inside of the `cmd` directory) is licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also included
in our repository in the `COPYING` file.
